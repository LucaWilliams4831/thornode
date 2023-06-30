import time
import codecs
import logging
import threading

from bitcointx import select_chain_params
from bitcointx.wallet import CBitcoinRegtestKey
from dogecointx.wallet import P2PKHDogecoinRegtestAddress
from utils.common import Coin, HttpClient, get_rune_asset, Asset
from decimal import Decimal, getcontext
from chains.aliases import aliases_doge, get_aliases, get_alias_address
from chains.chain import GenericChain
from tenacity import retry, stop_after_delay, wait_fixed

getcontext().prec = 8

RUNE = get_rune_asset()


class MockDogecoin(HttpClient):
    """
    An client implementation for a regtest dogecoin server
    """

    private_keys = [
        "ef235aacf90d9f4aadd8c92e4b2562e1d9eb97f0df9ba3b508258739cb013db2",
        "289c2857d4598e37fb9647507e47a309d6133539bf21a8b9cb6df88fd5232032",
        "e810f1d7d6691b4a7a73476f3543bd87d601f9a53e7faf670eac2c5b517d83bf",
        "a96e62ed3955e65be32703f12d87b6b5cf26039ecfa948dc5107a495418e5330",
        "9294f4d108465fd293f7fe299e6923ef71a77f2cb1eb6d4394839c64ec25d5c0",
    ]
    default_gas = 500000
    block_stats = {
        "tx_rate": default_gas,
        "tx_size": 1000,
    }

    def __init__(self, base_url):
        super().__init__(base_url)

        select_chain_params("dogecoin/regtest")

        for key in self.private_keys:
            seckey = CBitcoinRegtestKey.from_secret_bytes(
                codecs.decode(key, "hex_codec")
            )
            self.call("importprivkey", str(seckey))
        threading.Thread(target=self.scan_blocks, daemon=True).start()

    @classmethod
    def get_address_from_pubkey(cls, pubkey):
        """
        Get dogecoin address

        :param string pubkey: public key
        :returns: string encoded address
        """
        return str(P2PKHDogecoinRegtestAddress.from_pubkey(pubkey))

    def scan_blocks(self):
        while True:
            try:
                result = self.get_block()
                total = 0
                total_vsize = 0
                for tx_hash in result["tx"]:
                    tx = self.get_transaction(tx_hash)
                    if len(tx["vin"]) == 1 and "coinbase" in tx["vin"][0]:
                        for vout in tx["vout"]:
                            total += vout["value"]
                    else:
                        total_vsize += tx["vsize"]
                if total_vsize > 0:
                    amt = total - 10000
                    avg_fee_rate = int(amt * Coin.ONE / total_vsize)
                    if avg_fee_rate < MockDogecoin.default_gas:
                        avg_fee_rate = MockDogecoin.default_gas
                    self.block_stats["tx_rate"] = (
                        avg_fee_rate
                        // MockDogecoin.default_gas
                        * MockDogecoin.default_gas
                    )
            except Exception:
                continue
            finally:
                time.sleep(0.5)

    def get_block(self, block_height=None):
        """
        Get the block data for a height
        """
        if not block_height:
            block_height = self.get_block_height()
        block_hash = self.get_block_hash(block_height)
        return self.call("getblock", block_hash)

    def call(self, service, *args):
        payload = {
            "version": "1.0",
            "method": service,
            "params": args,
        }
        result = self.post("/", payload)
        if result.get("error"):
            raise result["error"]
        return result["result"]

    def set_vault_address(self, addr):
        """
        Set the vault bnb address
        """
        aliases_doge["VAULT"] = addr
        self.call("importaddress", addr)

    def get_block_height(self):
        """
        Get the current block height of dogecoin regtest
        """
        return self.call("getblockcount")

    def get_block_hash(self, block_height):
        """
        Get the block hash for a height
        """
        return self.call("getblockhash", int(block_height))

    def get_transaction(self, tx_hash):
        """
        Get the transaction data for a hash
        """
        return self.call("getrawtransaction", tx_hash, True)

    def estimate_smart_fee(self, nblocks=1):
        """
        Get the estimate smart fee for getting in n blocks
        """
        return self.call("estimatesmartfee", nblocks)

    def estimate_fee(self, nblocks=2):
        """
        Get the estimate smart fee for getting in n blocks
        """
        return self.call("estimatefee", nblocks)

    def wait_for_blocks(self, count):
        """
        Wait for the given number of blocks
        """
        start_block = self.get_block_height()
        for x in range(0, 30):
            time.sleep(1)
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
        Get DOGE balance for an address
        """
        unspents = self.call("listunspent", 1, 9999999, [address])
        return int(sum(Decimal(u["amount"]) for u in unspents) * Coin.ONE)

    @retry(stop=stop_after_delay(30), wait=wait_fixed(1))
    def wait_for_node(self):
        """
        Dogecoin regtest node is started with directly mining 100 blocks
        to be able to start handling transactions.
        It can take a while depending on the machine specs so we retry.
        """
        current_height = self.get_block_height()
        if current_height < 100:
            logging.warning("Dogecoin regtest starting, waiting")
            raise Exception

    def transfer(self, txn):
        """
        Make a transaction/transfer on regtest dogecoin
        """
        self.wait_for_node()

        if not isinstance(txn.coins, list):
            txn.coins = [txn.coins]

        if txn.to_address in get_aliases():
            txn.to_address = get_alias_address(txn.chain, txn.to_address)

        if txn.from_address in get_aliases():
            txn.from_address = get_alias_address(txn.chain, txn.from_address)

        # update memo with actual address (over alias name)
        for alias in get_aliases():
            chain = txn.chain
            asset = txn.get_asset_from_memo()
            if asset:
                chain = asset.get_chain()
            # we use RUNE BNB address to identify a cross chain liqudity provision
            if txn.memo.startswith("ADD"):
                chain = RUNE.get_chain()
            addr = get_alias_address(chain, alias)
            txn.memo = txn.memo.replace(alias, addr)

        # create transaction
        amount = float(txn.coins[0].amount / Coin.ONE)
        tx_out = {txn.to_address: amount}

        # get unspents UTXOs
        address = txn.from_address
        min_amount = float(amount + (self.default_gas / Coin.ONE))  # add more for fee
        unspents = self.call("listunspent", 1, 9999, [str(address)])
        if len(unspents) == 0:
            raise Exception(f"Cannot transfer. No DOGE UTXO available for {address}")

        # choose the first UTXO with enough amount
        unspent = {}
        for utxo in unspents:
            if float(utxo["amount"]) > min_amount:
                unspent = utxo

        tx_in = [{"txid": unspent["txid"], "vout": unspent["vout"]}]

        # create change output if needed
        amount_utxo = float(unspent["amount"])
        amount_change = Decimal(amount_utxo) - Decimal(min_amount)
        if amount_change > 0:
            if "SEED" in txn.memo:
                amount_change -= Decimal(self.default_gas / Coin.ONE)
            tx_out[txn.from_address] = round(float(amount_change), 8)

        tx_out["data"] = txn.memo.encode().hex()

        tx = self.call("createrawtransaction", tx_in, tx_out)
        tx = self.call("signrawtransaction", tx)
        txn.id = self.call("sendrawtransaction", tx["hex"]).upper()
        txn.gas = [Coin("DOGE.DOGE", self.default_gas)]


class Dogecoin(GenericChain):
    """
    A local simple implementation of dogecoin chain
    """

    name = "Dogecoin"
    chain = "DOGE"
    coin = Asset("DOGE.DOGE")
    rune_fee = 2000000

    @classmethod
    def _calculate_gas(cls, pool, txn):
        """
        Calculate gas according to RUNE thorchain fee
        1 RUNE / 2 in DOGE value
        """
        if pool is None:
            return Coin(cls.coin, MockDogecoin.default_gas)

        doge_amount = pool.get_rune_in_asset(int(cls.rune_fee / 2))
        return Coin(cls.coin, doge_amount)
