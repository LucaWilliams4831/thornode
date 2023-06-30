import os
import logging

import ecdsa

from terra_sdk.core import AccAddress, AccPubKey
from terra_sdk.client.lcd import LCDClient
from terra_sdk.core.fee import Fee
from terra_sdk.core.bech32 import get_bech
from terra_sdk.key.mnemonic import MnemonicKey as TerraMnemonicKey
from utils.msgs import MsgDeposit

from terra_sdk.client.lcd.api.tx import CreateTxOptions

from utils.segwit_addr import address_from_public_key
from utils.common import HttpClient, Coins, Coin, Asset
from chains.aliases import get_alias_address, get_aliases, get_alias
from chains.chain import GenericChain
from chains.account import Account

# Init logging
logging.basicConfig(
    format="%(levelname).1s[%(asctime)s] %(message)s",
    level=os.environ.get("LOGLEVEL", "INFO"),
)


# wallet helper functions
# Thanks to https://github.com/hukkinj1/cosmospy
def generate_wallet():
    privkey = ecdsa.SigningKey.generate(curve=ecdsa.SECP256k1).to_string().hex()
    pubkey = privkey_to_pubkey(privkey)
    address = address_from_public_key(pubkey)
    return {"private_key": privkey, "public_key": pubkey, "address": address}


def privkey_to_pubkey(privkey):
    privkey_obj = ecdsa.SigningKey.from_string(
        bytes.fromhex(privkey), curve=ecdsa.SECP256k1
    )
    pubkey_obj = privkey_obj.get_verifying_key()
    return pubkey_obj.to_string("compressed").hex()


def privkey_to_address(privkey):
    pubkey = privkey_to_pubkey(privkey)
    return address_from_public_key(pubkey)


# override mnemonickey class from Terra to get a thor address
class MnemonicKey(TerraMnemonicKey):
    @property
    def acc_address(self) -> AccAddress:
        """Thorchain Bech32 account address.
        Default derivation via :data:`public_key` is provided.

        Raises:
            ValueError: if Key was not initialized with proper public key

        Returns:
            AccAddress: account address
        """
        if not self.raw_address:
            raise ValueError("could not compute acc_address: missing raw_address")
        return AccAddress(get_bech("tthor", self.raw_address.hex()))

    @property
    def acc_pubkey(self) -> AccPubKey:
        """Thorchain Bech32 account pubkey.
        Default derivation via :data:`public_key` is provided.
        Raises:
            ValueError: if Key was not initialized with proper public key
        Returns:
            AccPubKey: account pubkey
        """
        if not self.raw_pubkey:
            raise ValueError("could not compute acc_pubkey: missing raw_pubkey")
        return AccPubKey(get_bech("tthorpub", self.raw_pubkey.hex()))


class MockThorchain(HttpClient):
    """
    A local simple implementation of thorchain chain
    """

    chain = "THOR"
    mnemonic = {
        "USER-1": "vintage announce rapid clip spare stomach matter camp noble habit "
        + "beef amateur chimney time fuel machine culture end toe oval isolate "
        + "laptop solar gift",
        "PROVIDER-1": "discover blue crunch cart club fish airport crazy roast hybrid "
        + "scheme picnic veteran mango beach narrow luxury glory dynamic crawl symbol "
        + "win sell dress",
        "PROVIDER-2": "sock true leave evil budget lonely foster danger reopen anxiety "
        + "dash naive list advance unhappy trust inmate culture bounce museum light "
        + "more pear story",
    }

    def __init__(self, base_url):
        self.base_url = base_url
        self.lcd_client = LCDClient(base_url, "localterra")
        self.lcd_client.chain_id = "thorchain"
        self.init_wallets()

    def init_wallets(self):
        """
        Init wallet instances
        """
        self.wallets = {}
        for alias in self.mnemonic:
            mk = MnemonicKey(mnemonic=self.mnemonic[alias], coin_type=118)
            self.wallets[alias] = self.lcd_client.wallet(mk)

    def get_balance(self, address, asset=Asset("THOR.RUNE")):
        """
        Get THOR balance for an address
        """
        if "VAULT" == get_alias("THOR", address):
            balance = self.fetch("/thorchain/balance/module/asgard")
            for coin in balance:
                if coin["denom"] == asset.get_symbol().lower():
                    return int(coin["amount"])
        else:
            result = self.fetch("/cosmos/bank/v1beta1/balances/" + address)
            for coin in result["balances"]:
                if coin["denom"] == asset.get_symbol().lower():
                    return int(coin["amount"])
        return 0

    def transfer(self, txns):
        if not isinstance(txns, list):
            txns = [txns]

        for txn in txns:
            if not isinstance(txn.coins, list):
                txn.coins = [txn.coins]
            wallet = self.wallets[txn.from_address]
            txn.gas = [Coin("THOR.RUNE", 2000000)]
            if txn.from_address in get_aliases():
                txn.from_address = get_alias_address(txn.chain, txn.from_address)
            if txn.to_address in get_aliases():
                txn.to_address = get_alias_address(txn.chain, txn.to_address)

            # update memo with actual address (over alias name)
            is_synth = txn.is_synth()
            for alias in get_aliases():
                chain = txn.chain
                asset = txn.get_asset_from_memo()
                if asset and not is_synth:
                    chain = asset.get_chain()
                addr = get_alias_address(chain, alias)
                txn.memo = txn.memo.replace(alias, addr)

            coins = Coins(txn.coins)
            tx_options = CreateTxOptions(
                msgs=[MsgDeposit(coins, txn.memo, txn.from_address)],
                fee=Fee(20000000, "0urune"),  # gas limit 0.2urune fee 0urune,
            )
            tx = wallet.create_and_sign_tx(tx_options)
            result = self.lcd_client.tx.broadcast(tx)
            if result.code:
                raise Exception(result)
            txn.id = result.txhash


class Thorchain(GenericChain):
    """
    A local simple implementation of thorchain chain
    """

    name = "THORChain"
    chain = "THOR"
    coin = Asset("THOR.RUNE")

    def __init__(self):
        super().__init__()

        # seeding the users, these seeds are established in build/scripts/genesis.sh
        acct = Account("tthor1z63f3mzwv3g75az80xwmhrawdqcjpaekk0kd54")
        acct.add(Coin(self.coin, 5000000000000))
        self.set_account(acct)

        acct = Account("tthor1wz78qmrkplrdhy37tw0tnvn0tkm5pqd6zdp257")
        acct.add(Coin(self.coin, 25000000000100))
        self.set_account(acct)

        acct = Account("tthor1xwusttz86hqfuk5z7amcgqsg7vp6g8zhsp5lu2")
        acct.add(Coin(self.coin, 5090000000000))
        self.set_account(acct)

    @classmethod
    def _calculate_gas(cls, pool, txn):
        """
        With given coin set, calculates the gas owed
        """
        return Coin(cls.coin, 2000000)
