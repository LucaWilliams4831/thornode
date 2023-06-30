import time
import base64
import hashlib

from utils.common import Coin, HttpClient, get_rune_asset, Asset
from utils.segwit_addr import address_from_public_key
from chains.aliases import aliases_bnb, get_aliases, get_alias_address
from chains.chain import GenericChain

RUNE = get_rune_asset()


class BinanceApi(HttpClient):
    """
    An client implementation for a Binance API server
    """

    def account(self, address):
        return self.fetch(f"/api/v1/account/{address}")


class MockBinance(HttpClient):
    """
    An client implementation for a mock binance server
    https://gitlab.com/thorchain/bepswap/mock-binance
    """

    singleton_gas = 37500
    multi_gas = 30000

    def set_vault_address_by_pubkey(self, pubkey):
        """
        Set vault adddress by pubkey
        """
        self.set_vault_address(self.get_address_from_pubkey(pubkey))

    def set_vault_address(self, addr):
        """
        Set the vault bnb address
        """
        aliases_bnb["VAULT"] = addr

    def get_block_height(self):
        """
        Get the current block height of mock binance
        """
        data = self.fetch("/block")
        return int(data["result"]["block"]["header"]["height"])

    def get_block_tx(self, height):
        """
        Get the current block tx from height of mock binance
        """
        data = self.fetch(f"/block?height={height}")
        return data["result"]["block"]["data"]["txs"][0]

    def wait_for_blocks(self, count):
        """
        Wait for the given number of blocks
        """
        start_block = self.get_block_height()
        for x in range(0, 30):
            time.sleep(0.3)
            block = self.get_block_height()
            if block - start_block >= count:
                return

    def get_tx_id_from_block(self, height):
        """Get transaction hash ID from a block height.
        We first retrieve tx data from block then generate id from tx data:
        raw tx base 64 encoded -> base64 decode -> sha256sum = tx hash

        :param str height: block height
        :returns: tx hash id hex string

        """
        tx = self.get_block_tx(height)
        decoded = base64.b64decode(tx)
        return hashlib.new("sha256", decoded).digest().hex().upper()

    def accounts(self):
        return self.fetch("/accounts")

    @classmethod
    def get_address_from_pubkey(cls, pubkey, prefix="tbnb"):
        """
        Get bnb testnet address for a public key

        :param string pubkey: public key
        :returns: string bech32 encoded address
        """
        return address_from_public_key(pubkey, prefix)

    def transfer(self, txns):
        """
        Make a transaction/transfer on mock binance
        """
        if not isinstance(txns, list):
            txns = [txns]

        payload = []
        for txn in txns:
            if not isinstance(txn.coins, list):
                txn.coins = [txn.coins]

            if txn.to_address in get_aliases():
                txn.to_address = get_alias_address(txn.chain, txn.to_address)

            if txn.from_address in get_aliases():
                txn.from_address = get_alias_address(txn.chain, txn.from_address)

            # update memo with actual address (over alias name)
            is_synth = txn.is_synth()
            for alias in get_aliases():
                chain = txn.chain
                asset = txn.get_asset_from_memo()
                if asset:
                    chain = asset.get_chain()
                if is_synth:
                    chain = RUNE.get_chain()
                if txn.memo.startswith("ADD"):
                    if asset and txn.chain == asset.get_chain():
                        chain = RUNE.get_chain()
                addr = get_alias_address(chain, alias)
                txn.memo = txn.memo.replace(alias, addr)

            payload.append(
                {
                    "from": txn.from_address,
                    "to": txn.to_address,
                    "memo": txn.memo,
                    "coins": [coin.to_binance_fmt() for coin in txn.coins],
                }
            )
        result = self.post("/broadcast/easy", payload)
        txn.id = self.get_tx_id_from_block(result["height"])


class Binance(GenericChain):
    """
    A local simple implementation of binance chain
    """

    name = "Binance"
    chain = "BNB"
    coin = Asset("BNB.BNB")

    @classmethod
    def _calculate_gas(cls, pool, txn):
        """
        With given coin set, calculates the gas owed
        """
        if not isinstance(txn.coins, list) or len(txn.coins) == 1:
            return Coin(cls.coin, MockBinance.singleton_gas)
        return Coin(cls.coin, MockBinance.multi_gas * len(txn.coins))
