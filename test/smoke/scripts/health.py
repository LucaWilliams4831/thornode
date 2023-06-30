import argparse
import sys
import os
import logging

from decimal import Decimal, getcontext

from chains.binance import MockBinance, BinanceApi
from chains.account import Account
from thorchain.thorchain import ThorchainClient
from thorchain.midgard import MidgardClient
from utils.common import Coin, get_rune_asset, get_diff
from utils.segwit_addr import decode_address

# Init logging
logging.basicConfig(
    format="%(levelname).1s[%(asctime)s] %(message)s",
    level=os.environ.get("LOGLEVEL", "INFO"),
)

getcontext().prec = 20

RUNE = get_rune_asset()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--thorchain", default="http://localhost:1317", help="Thorchain API url"
    )
    parser.add_argument(
        "--midgard", default="http://localhost:8080", help="Midgard API url"
    )
    parser.add_argument(
        "--binance", default="http://localhost:26660", help="Mock Binance server"
    )
    parser.add_argument("--binance-api", default=None, help="Binance API server")
    parser.add_argument(
        "--check-balance", default=True, type=bool, help="Check vault balances"
    )
    parser.add_argument(
        "--margin-err", default=0, type=float, help="Allow margin of error in %"
    )

    args = parser.parse_args()

    health = Health(
        args.thorchain,
        args.midgard,
        args.binance,
        args.check_balance,
        args.margin_err,
        args.binance_api,
    )
    try:
        health.run()
        sys.exit(health.exit)
    except Exception as e:
        logging.error(e)
        sys.exit(1)


class Health:
    def __init__(
        self,
        thor,
        midgard,
        binance,
        check_balance=True,
        margin_err=0,
        binance_api=None,
        fast_fail=False,
    ):
        self.thorchain_client = ThorchainClient(thor)
        self.thorchain_pools = []
        self.thorchain_asgard_vaults = []

        self.midgard_client = MidgardClient(midgard)
        self.midgard_pools = []
        self.check_balance = check_balance
        self.margin_err = margin_err

        self.binance_client = MockBinance(binance)
        self.binance_api = BinanceApi(binance_api)
        self.binance_accounts = []
        self.fast_fail = fast_fail
        self.exit = 0

    def run(self):
        """Run health checks

        - check pools state between midgard and thorchain

        """
        self.check_pools()
        if self.check_balance:
            self.retrieve_vaults()
            self.check_asgard_vaults()
            self.check_yggdrasil_vaults()

    def error(self, err):
        """Check errors and exit accordingly."""
        self.exit = 1
        if self.fast_fail:
            raise Exception(err)
        else:
            logging.error(err)

    def retrieve_vaults(self):
        """Retrieve vault data from THORChain APIs."""
        self.thorchain_asgards = self.thorchain_client.get_asgard_vaults()
        for vault in self.thorchain_asgards:
            if vault["coins"]:
                vault["coins"] = [Coin.from_data(c) for c in vault["coins"]]

        self.thorchain_yggdrasils = self.thorchain_client.get_yggdrasil_vaults()
        for vault in self.thorchain_yggdrasils:
            if vault["coins"]:
                vault["coins"] = [Coin.from_data(c) for c in vault["coins"]]

        if not self.binance_api.base_url:
            self.binance_accounts = []
            accounts = self.binance_client.accounts()
            for acct in accounts:
                account = Account(acct["address"])
                if acct["balances"]:
                    account.balances = [
                        Coin(b["denom"], b["amount"]) for b in acct["balances"]
                    ]
                    self.binance_accounts.append(account)

    def check_pools(self):
        """Check pools state between Midgard and Thorchain APIs."""
        self.thorchain_pools = self.thorchain_client.get_pools()
        for tpool in self.thorchain_pools:
            asset = tpool["asset"]
            mpool = self.midgard_client.get_pool(asset)

            # Thorchain Coins
            trune = Coin(RUNE, tpool["balance_rune"])
            tasset = Coin(asset, tpool["balance_asset"])
            tsynth = Coin(asset, tpool["synth_supply"])

            # Midgard Coins
            mrune = Coin(RUNE, mpool["runeDepth"])
            masset = Coin(asset, mpool["assetDepth"])
            msynth = Coin(asset, mpool["synthSupply"])

            # Check balances
            diff = get_diff(trune.amount, mrune.amount)
            if diff > self.margin_err:
                sub = abs(trune - mrune)
                self.error(
                    f"Midgard   [{asset:15}] RUNE "
                    f" [T]{trune.str_amt()} != [M]{mrune.str_amt()}"
                    f" [DIFF] {str(round(diff, 5))}% ({sub:0,.0f})"
                )

            diff = get_diff(tasset.amount, masset.amount)
            if diff > self.margin_err:
                sub = abs(tasset - masset)
                self.error(
                    f"Midgard   [{asset:15}] ASSET"
                    f" [T]{tasset.str_amt()} != [M]{masset.str_amt()}"
                    f" [DIFF] {str(round(diff, 5))}% ({sub:0,.0f})"
                )

            diff = get_diff(tsynth.amount, msynth.amount)
            if diff > self.margin_err:
                sub = abs(tsynth - msynth)
                self.error(
                    f"Midgard   [{asset:15}] SYNTH SUPPLY"
                    f" [T]{tsynth.str_amt()} != [M]{msynth.str_amt()}"
                    f" [DIFF] {str(round(diff, 5))}% ({sub:0,.0f})"
                )

            # Check pool units
            mpool_units = int(mpool["units"])
            tpool_units = int(tpool["pool_units"])
            diff = get_diff(mpool_units, tpool_units)
            if diff > self.margin_err:
                sub = abs(tpool_units - mpool_units)
                self.error(
                    f"Midgard   [{asset:15}] POOL UNITS"
                    f" [T]{tpool_units:0,.0f} != [M]{mpool_units:0,.0f}"
                    f" [DIFF] {str(round(diff, 5))}% ({sub:0,.0f})"
                )

            # Check pool LP units
            mpool_units = int(mpool["liquidityUnits"])
            tpool_units = int(tpool["LP_units"])
            diff = get_diff(mpool_units, tpool_units)
            if diff > self.margin_err:
                sub = abs(tpool_units - mpool_units)
                self.error(
                    f"Midgard   [{asset:15}] LP UNITS"
                    f" [T]{tpool_units:0,.0f} != [M]{mpool_units:0,.0f}"
                    f" [DIFF] {str(round(diff, 5))}% ({sub:0,.0f})"
                )

            # Check pool synth units
            mpool_units = int(mpool["synthUnits"])
            tpool_units = int(tpool["synth_units"])
            diff = get_diff(mpool_units, tpool_units)
            if diff > self.margin_err:
                sub = abs(tpool_units - mpool_units)
                self.error(
                    f"Midgard   [{asset:15}] SYNTH UNITS"
                    f" [T]{tpool_units:0,.0f} != [M]{mpool_units:0,.0f}"
                    f" [DIFF] {str(round(diff, 5))}% ({sub:0,.0f})"
                )

            # Check price
            mpool_price = float(mpool["assetPrice"])
            tpool_price = int(tpool["balance_rune"]) / int(tpool["balance_asset"])
            diff = get_diff(mpool_price, tpool_price)
            if diff > self.margin_err:
                sub = abs(tpool_price - mpool_price)
                self.error(
                    f"Midgard   [{asset:15}] PRICE"
                    f" [T]{tpool_price:0,.8f} != [M]{mpool_price:0,.8f}"
                    f" [DIFF] {str(round(diff, 5))}% ({sub:0,.8f})"
                )

    def check_binance_account(self, vault, vault_type):
        """Check a vault balances against Binance."""
        # get raw pubkey from bech32 + amino encoded key
        # we need to get rid of the 5 first bytes used in amino encoding
        pub_key = decode_address(vault["pub_key"])[5:]

        if self.binance_api.base_url:
            # TESTNET and CHAOSNET scenarios, check real balance
            if "testnet" in self.binance_api.base_url:
                prefix = "tbnb"
            else:
                prefix = "bnb"
            vault_addr = MockBinance.get_address_from_pubkey(pub_key, prefix)
            acct = self.binance_api.account(vault_addr)
            account = Account(vault_addr)
            if acct["balances"]:
                account.balances = []
                for b in acct["balances"]:
                    symbol = f"BNB.{b['symbol']}"
                    amount = Decimal(b["free"]) * Coin.ONE
                    account.balances.append(Coin(symbol, amount))
            for bcoin in account.balances:
                for vcoin in vault["coins"]:
                    if vcoin.asset != bcoin.asset:
                        continue
                    diff = get_diff(vcoin, bcoin)
                    if diff > self.margin_err:
                        sub = abs(vcoin - bcoin)
                        self.error(
                            f"{vault_type} [{vcoin.asset:15}]"
                            f" [T]{vcoin.str_amt()} != [B]{bcoin.str_amt()}"
                            f" [DIFF] {str(round(diff, 5))}% ({sub:0,.0f})"
                        )
        else:
            # MOCKNET scenario check accounts from mock-binance
            vault_addr = MockBinance.get_address_from_pubkey(pub_key)
            for acct in self.binance_accounts:
                if acct.address != vault_addr:
                    continue
                for bcoin in acct.balances:
                    for vcoin in vault["coins"]:
                        if vcoin.asset != bcoin.asset:
                            continue
                        diff = get_diff(vcoin, bcoin)
                        if diff > self.margin_err:
                            sub = abs(vcoin - bcoin)
                            self.error(
                                f"{vault_type} [{vcoin.asset:15}]"
                                f" [T]{vcoin.str_amt()} != [B]{bcoin.str_amt()}"
                                f" [DIFF] {str(round(diff, 5))}% ({sub:0,.0f})"
                            )

    def check_asgard_vaults(self):
        """Check Asgard vaults balances against Binance balances."""
        for vault in self.thorchain_asgards:
            self.check_binance_account(vault, "Asgard   ")

    def check_yggdrasil_vaults(self):
        """Check Yggdrasil vaults balances against Binance balances."""
        for vault in self.thorchain_yggdrasils:
            self.check_binance_account(vault, "Yggdrasil")


if __name__ == "__main__":
    main()
