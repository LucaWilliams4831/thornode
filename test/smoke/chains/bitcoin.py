import os
import time
import codecs
import logging
import threading
import durationpy

from bitcointx import select_chain_params
from bitcointx.wallet import CBitcoinRegtestKey, P2WPKHBitcoinRegtestAddress
from utils.common import Coin, HttpClient, get_rune_asset, Asset
from decimal import Decimal, getcontext
from chains.aliases import aliases_btc, get_aliases, get_alias_address
from chains.chain import GenericChain
from tenacity import retry, stop_after_delay, wait_fixed

getcontext().prec = 8

RUNE = get_rune_asset()


class MockBitcoin(HttpClient):
    """
    An client implementation for a regtest bitcoin server
    """

    private_keys = [
        "ef235aacf90d9f4aadd8c92e4b2562e1d9eb97f0df9ba3b508258739cb013db2",
        "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032",
        "e810f1d7d6691b4a7a73476f3543bd87d601f9a53e7faf670eac2c5b517d83bf",
        "a96e62ed3955e65be32703f12d87b6b5cf26039ecfa948dc5107a495418e5330",
        "9294f4d108465fd293f7fe299e6923ef71a77f2cb1eb6d4394839c64ec25d5c0",
    ]
    default_gas = 100000
    block_stats = {
        "tx_rate": 0,
        "tx_size": 0,
    }

    def __init__(self, base_url):
        super().__init__(base_url)
        select_chain_params("bitcoin/regtest")
        self.wait_for_node()
        self.create_wallet()
        for key in self.private_keys:
            seckey = CBitcoinRegtestKey.from_secret_bytes(
                codecs.decode(key, "hex_codec")
            )
            self.call("importprivkey", str(seckey))
        threading.Thread(target=self.scan_blocks, daemon=True).start()

    @retry(stop=stop_after_delay(30), wait=wait_fixed(1))
    def create_wallet(self):
        wallets = self.call("listwallets")
        if len(wallets) == 0:
            self.call("createwallet", "", "false", "false", "", "false", "false")

    def scan_blocks(self):
        while True:
            try:
                result = self.get_block_stats()
                avg_fee_rate = result["avgfeerate"]
                avg_tx_size = 1000  # result["mediantxsize"]
                if avg_fee_rate != 0:
                    min_relay_fee = 1000  # sats
                    if avg_fee_rate * avg_tx_size < min_relay_fee:
                        avg_fee_rate = 4  # min_relay_fee / avg_tx_size
                    self.block_stats["tx_rate"] = avg_fee_rate
                    self.block_stats["tx_size"] = avg_tx_size
            except Exception:
                continue
            finally:
                default = "1.0s"
                backoff = os.environ.get("BLOCK_SCANNER_BACKOFF", default)
                if backoff == "" or backoff is None:
                    backoff = default
                backoff = durationpy.from_str(backoff).total_seconds()
                time.sleep(backoff)

    @classmethod
    def get_address_from_pubkey(cls, pubkey):
        """
        Get bitcoin address for a specific hrp (human readable part)
        bech32 encoded from a public key(secp256k1).

        :param string pubkey: public key
        :returns: string bech32 encoded address
        """
        return str(P2WPKHBitcoinRegtestAddress.from_pubkey(pubkey))

    def call(self, service, *args):
        payload = {
            "version": "1.1",
            "method": service,
            "params": args,
        }
        result = self.post("/wallet/", payload)
        if result.get("error"):
            raise result["error"]
        return result["result"]

    def set_vault_address(self, addr):
        """
        Set the vault bnb address
        """
        aliases_btc["VAULT"] = addr
        self.call("importaddress", addr)

    def get_block_height(self):
        """
        Get the current block height of bitcoin regtest
        """
        return self.call("getblockcount")

    def get_block_hash(self, block_height):
        """
        Get the block hash for a height
        """
        return self.call("getblockhash", int(block_height))

    def get_block_stats(self, block_height=None):
        """
        Get the block hash for a height
        """
        if not block_height:
            block_height = self.get_block_height()
        return self.call("getblockstats", int(block_height))

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

    def invalidate_block(self, block_hash):
        """
        Invalidate a block
        """
        self.call("invalidateblock", block_hash)

    def get_balance(self, address):
        """
        Get BTC balance for an address
        """
        unspents = self.call("listunspent", 1, 9999999, [address])
        return int(sum(Decimal(u["amount"]) for u in unspents) * Coin.ONE)

    @retry(stop=stop_after_delay(30), wait=wait_fixed(1))
    def wait_for_node(self):
        """
        Bitcoin regtest node is started with directly mining 100 blocks
        to be able to start handling transactions.
        It can take a while depending on the machine specs so we retry.
        """
        current_height = self.get_block_height()
        if current_height < 100:
            logging.warning("Bitcoin regtest starting, waiting")
            raise Exception

    def transfer(self, txn):
        """
        Make a transaction/transfer on regtest bitcoin
        """
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
            # we use RUNE BNB address to identify a cross chain liqudity provision
            if txn.memo.startswith("ADD") or is_synth:
                chain = RUNE.get_chain()
            addr = get_alias_address(chain, alias)
            txn.memo = txn.memo.replace(alias, addr)

        # create transaction
        amount = float(txn.coins[0].amount / Coin.ONE)
        tx_out_dest = {txn.to_address: amount}
        tx_out_op_return = {"data": txn.memo.encode().hex()}

        # get unspents UTXOs
        address = txn.from_address
        min_amount = float(amount + (self.default_gas / Coin.ONE))  # add more for fee
        unspents = self.call(
            "listunspent", 1, 9999, [str(address)], True, {"minimumAmount": min_amount}
        )
        if len(unspents) == 0:
            raise Exception(f"Cannot transfer. No BTC UTXO available for {address}")

        # choose the first UTXO
        unspent = unspents[0]
        tx_in = [{"txid": unspent["txid"], "vout": unspent["vout"]}]
        tx_out = [tx_out_dest]

        # create change output if needed
        amount_utxo = float(unspent["amount"])
        amount_change = Decimal(amount_utxo) - Decimal(min_amount)
        if amount_change > 0:
            if "SEED" in txn.memo:
                amount_change -= Decimal(self.default_gas / Coin.ONE)
            tx_out.append({txn.from_address: round(float(amount_change), 8)})

        tx_out.append(tx_out_op_return)

        tx = self.call("createrawtransaction", tx_in, tx_out)
        tx = self.call("signrawtransactionwithwallet", tx)
        txn.id = self.call("sendrawtransaction", tx["hex"]).upper()
        txn.gas = [Coin("BTC.BTC", self.default_gas)]


class Bitcoin(GenericChain):
    """
    A local simple implementation of bitcoin chain
    """

    name = "Bitcoin"
    chain = "BTC"
    coin = Asset("BTC.BTC")
    rune_fee = 2000000

    @classmethod
    def _calculate_gas(cls, pool, txn):
        """
        Calculate gas according to RUNE thorchain fee
        1 RUNE / 2 in BTC value
        """
        if pool is None:
            return Coin(cls.coin, MockBitcoin.default_gas)

        btc_amount = pool.get_rune_in_asset(int(cls.rune_fee / 2))
        return Coin(cls.coin, btc_amount)
