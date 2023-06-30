import unittest
import json

from copy import deepcopy
from utils.common import (
    Asset,
    Transaction,
    Coin,
    get_share,
    get_rune_asset,
    DEFAULT_RUNE_ASSET,
)
from chains.binance import Binance

RUNE = get_rune_asset()


class TestAsset(unittest.TestCase):
    def test_constructor(self):
        asset = Asset("BNB.BNB")
        self.assertEqual(asset, "BNB.BNB")
        asset = Asset("BNB")
        self.assertEqual(asset, "THOR.BNB")
        asset = Asset(RUNE)
        self.assertEqual(asset, RUNE)
        asset = Asset(RUNE)
        self.assertEqual(asset, RUNE)
        asset = Asset("BNB.LOK-3C0")
        self.assertEqual(asset, "BNB.LOK-3C0")

    def test_get_share(self):
        alloc = 50000000
        part = 149506590
        total = 50165561086
        share = get_share(part, total, alloc)
        self.assertEqual(share, 149013)

    def test_get_symbol(self):
        asset = Asset("BNB.BNB")
        self.assertEqual(asset.get_symbol(), "BNB")
        asset = Asset(RUNE)
        self.assertEqual(asset.get_symbol(), RUNE.get_symbol())
        asset = Asset("LOK-3C0")
        self.assertEqual(asset.get_symbol(), "LOK-3C0")

    def test_get_ticker(self):
        asset = Asset("BNB.BNB")
        self.assertEqual(asset.get_ticker(), "BNB")
        asset = Asset(RUNE)
        self.assertEqual(asset.get_ticker(), "RUNE")
        asset = Asset("LOK-3C0")
        self.assertEqual(asset.get_ticker(), "LOK")

    def test_get_chain(self):
        asset = Asset("BNB.BNB")
        self.assertEqual(asset.get_chain(), "BNB")
        asset = Asset(RUNE)
        self.assertEqual(asset.get_chain(), RUNE.get_chain())
        asset = Asset("LOK-3C0")
        self.assertEqual(asset.get_chain(), "THOR")

    def test_is_rune(self):
        asset = Asset("BNB.BNB")
        self.assertEqual(asset.is_rune(), False)
        asset = Asset(RUNE)
        self.assertEqual(asset.is_rune(), True)
        asset = Asset("LOK-3C0")
        self.assertEqual(asset.is_rune(), False)
        asset = Asset("RUNE")
        self.assertEqual(asset.is_rune(), True)

    def test_to_json(self):
        asset = Asset("BNB.BNB")
        self.assertEqual(asset.to_json(), json.dumps("BNB.BNB"))
        asset = Asset("BNB.LOK-3C0")
        self.assertEqual(asset.to_json(), json.dumps("BNB.LOK-3C0"))
        asset = Asset(RUNE)
        self.assertEqual(asset.to_json(), json.dumps(RUNE))


class TestCoin(unittest.TestCase):
    def test_constructor(self):
        coin = Coin("BNB.BNB", 100)
        self.assertEqual(coin.asset, "BNB.BNB")
        self.assertEqual(coin.amount, 100)
        coin = Coin("BNB.BNB")
        self.assertEqual(coin.asset, "BNB.BNB")
        self.assertEqual(coin.amount, 0)
        coin = Coin(RUNE, 1000000)
        self.assertEqual(coin.amount, 1000000)
        self.assertEqual(coin.asset, RUNE)

        coin = Coin(RUNE, 400_000 * 100000000)
        c = coin.__dict__
        self.assertEqual(c["amount"], 400_000 * 100000000)
        self.assertEqual(c["asset"], RUNE)

    def test_is_zero(self):
        coin = Coin("BNB.BNB", 100)
        self.assertEqual(coin.is_zero(), False)
        coin = Coin("BNB.BNB")
        self.assertEqual(coin.is_zero(), True)
        coin = Coin(RUNE, 0)
        self.assertEqual(coin.is_zero(), True)

    def test_eq(self):
        coin1 = Coin("BNB.BNB", 100)
        coin2 = Coin("BNB.BNB")
        self.assertNotEqual(coin1, coin2)
        coin1 = Coin("BNB.BNB", 100)
        coin2 = Coin("BNB.BNB", 100)
        self.assertEqual(coin1, coin2)
        coin1 = Coin("BNB.LOK-3C0", 100)
        coin2 = Coin(RUNE, 100)
        self.assertNotEqual(coin1, coin2)
        coin1 = Coin("BNB.LOK-3C0", 100)
        coin2 = Coin("BNB.LOK-3C0", 100)
        self.assertEqual(coin1, coin2)
        coin1 = Coin("LOK-3C0", 200)
        coin2 = Coin("LOK-3C0", 200)
        self.assertEqual(coin1, coin2)
        coin1 = Coin("RUNE")
        coin2 = Coin("RUNE")
        self.assertEqual(coin1, coin2)
        # check list equality
        list1 = [Coin("RUNE", 100), Coin("RUNE", 100)]
        list2 = [Coin("RUNE", 100), Coin("RUNE", 100)]
        self.assertEqual(list1, list2)
        list1 = [Coin("RUNE", 100), Coin("RUNE", 100)]
        list2 = [Coin("RUNE", 10), Coin("RUNE", 100)]
        self.assertNotEqual(list1, list2)
        # list not sorted are NOT equal
        list1 = [Coin("RUNE", 100), Coin("BNB.BNB", 200)]
        list2 = [Coin("BNB.BNB", 200), Coin("RUNE", 100)]
        self.assertNotEqual(list1, list2)
        self.assertEqual(sorted(list1), sorted(list2))
        # check sets
        list1 = [Coin("RUNE", 100), Coin("RUNE", 100)]
        self.assertEqual(len(set(list1)), 1)
        list1 = [Coin("RUNE", 100), Coin("RUNE", 10)]
        self.assertEqual(len(set(list1)), 2)

    def test_is_rune(self):
        coin = Coin("BNB.BNB")
        self.assertEqual(coin.is_rune(), False)
        coin = Coin(RUNE)
        self.assertEqual(coin.is_rune(), True)
        coin = Coin("LOK-3C0")
        self.assertEqual(coin.is_rune(), False)
        coin = Coin("RUNE")
        self.assertEqual(coin.is_rune(), True)

    def test_to_binance_fmt(self):
        coin = Coin("BNB.BNB")
        self.assertEqual(coin.to_binance_fmt(), {"denom": "BNB", "amount": 0})
        coin = Coin("RUNE", 1000000)
        self.assertEqual(coin.to_binance_fmt(), {"denom": "RUNE", "amount": 1000000})
        coin = Coin("LOK-3C0", 1000000)
        self.assertEqual(coin.to_binance_fmt(), {"denom": "LOK-3C0", "amount": 1000000})

    def test_str(self):
        coin = Coin("BNB.BNB")
        self.assertEqual(str(coin), "0.00000000 BNB.BNB")
        coin = Coin(RUNE, 1000000)
        self.assertEqual(str(coin), "0.01000000 " + RUNE)
        coin = Coin("BNB.LOK-3C0", 1000000)
        self.assertEqual(str(coin), "0.01000000 BNB.LOK-3C0")

    def test_repr(self):
        coin = Coin("BNB.BNB")
        self.assertEqual(repr(coin), "<Coin 0.00000000 BNB.BNB>")
        coin = Coin(RUNE, 1000000)
        self.assertEqual(repr(coin), f"<Coin 0.01000000 {RUNE}>")
        coin = Coin("BNB.LOK-3C0", 1000000)
        self.assertEqual(repr(coin), "<Coin 0.01000000 BNB.LOK-3C0>")

    def test_to_json(self):
        coin = Coin("BNB.BNB")
        self.assertEqual(coin.to_json(), '{"asset": "BNB.BNB", "amount": 0}')
        coin = Coin(RUNE, 1000000)
        self.assertEqual(coin.to_json(), '{"asset": "' + RUNE + '", "amount": 1000000}')
        coin = Coin("BNB.LOK-3C0", 1000000)
        self.assertEqual(coin.to_json(), '{"asset": "BNB.LOK-3C0", "amount": 1000000}')

    def test_from_data(self):
        value = {
            "asset": "BNB.BNB",
            "amount": 1000,
        }
        coin = Coin.from_data(value)
        self.assertEqual(coin.asset, "BNB.BNB")
        self.assertEqual(coin.amount, 1000)
        value = {
            "asset": RUNE,
            "amount": "1000",
        }
        coin = Coin.from_data(value)
        self.assertEqual(coin.asset, RUNE)
        self.assertEqual(coin.amount, 1000)


class TestTransaction(unittest.TestCase):
    def test_constructor(self):
        txn = Transaction(
            Binance.chain,
            "USER",
            "VAULT",
            Coin("BNB.BNB", 100),
            "MEMO",
        )
        self.assertEqual(txn.chain, "BNB")
        self.assertEqual(txn.from_address, "USER")
        self.assertEqual(txn.to_address, "VAULT")
        self.assertEqual(txn.coins[0].asset, "BNB.BNB")
        self.assertEqual(txn.coins[0].amount, 100)
        self.assertEqual(txn.memo, "MEMO")
        txn.coins = [Coin("BNB.BNB", 1000000000), Coin(RUNE, 1000000000)]
        self.assertEqual(txn.coins[0].asset, "BNB.BNB")
        self.assertEqual(txn.coins[0].amount, 1000000000)
        self.assertEqual(txn.coins[1].asset, RUNE)
        self.assertEqual(txn.coins[1].amount, 1000000000)

    def test_str(self):
        txn = Transaction(
            Binance.chain,
            "USER",
            "VAULT",
            Coin("BNB.BNB", 100),
            "MEMO",
        )
        self.assertEqual(str(txn), "      USER => VAULT      [MEMO] 0.00000100 BNB.BNB")
        txn.coins = [Coin("BNB.BNB", 1000000000), Coin(RUNE, 1000000000)]
        self.assertEqual(
            str(txn),
            "      USER => VAULT      [MEMO] 10.00000000 BNB.BNB"
            f", 10.00000000 {RUNE}",
        )
        txn.coins = None
        self.assertEqual(
            str(txn),
            "      USER => VAULT      [MEMO] No Coins",
        )
        txn.gas = [Coin("BNB.BNB", 37500)]
        self.assertEqual(
            str(txn),
            "      USER => VAULT      [MEMO] No Coins | Gas 0.00037500 BNB.BNB",
        )

    def test_repr(self):
        txn = Transaction(
            Binance.chain,
            "USER",
            "VAULT",
            Coin("BNB.BNB", 100),
            "MEMO",
        )
        self.assertEqual(
            repr(txn),
            "<Tx       USER => VAULT      [MEMO] [<Coin 0.00000100 BNB.BNB>]>",
        )
        txn.coins = [Coin("BNB.BNB", 1000000000), Coin(RUNE, 1000000000)]
        self.assertEqual(
            repr(txn),
            "<Tx       USER => VAULT      [MEMO] [<Coin 10.00000000 BNB.BNB>,"
            f" <Coin 10.00000000 {RUNE}>]>",
        )
        txn.coins = None
        self.assertEqual(repr(txn), "<Tx       USER => VAULT      [MEMO] No Coins>")
        txn.gas = [Coin("BNB.BNB", 37500)]
        self.assertEqual(
            repr(txn),
            "<Tx       USER => VAULT      [MEMO] No Coins |"
            " Gas [<Coin 0.00037500 BNB.BNB>]>",
        )

    def test_is_cross_chain_provision(self):
        tx = Transaction(
            Binance.chain,
            "USER",
            "VAULT",
            Coin("BNB.BNB", 100),
            "ADD:BNB.BNB:PROVIDER-1",
        )
        self.assertEqual(tx.is_cross_chain_provision(), True)
        tx = Transaction(
            "THOR",
            "USER",
            "VAULT",
            Coin("THOR.RUNE", 100),
            "ADD:BNB.BNB:PROVIDER-1",
        )
        self.assertEqual(tx.is_cross_chain_provision(), True)
        tx = Transaction(
            "THOR",
            "USER",
            "VAULT",
            Coin("THOR.RUNE", 100),
            "ADD:",
        )
        self.assertEqual(tx.is_cross_chain_provision(), False)

    def test_eq(self):
        tx1 = Transaction(
            Binance.chain,
            "USER",
            "VAULT",
            Coin("BNB.BNB", 100),
            "ADD:BNB",
        )
        tx2 = Transaction(
            Binance.chain,
            "USER",
            "VAULT",
            Coin("BNB.BNB", 100),
            "ADD:BNB",
        )
        self.assertEqual(tx1, tx2)
        tx2.chain = "BTC"
        self.assertNotEqual(tx1, tx2)
        tx1 = Transaction(
            Binance.chain,
            "USER",
            "VAULT",
            [Coin("BNB.BNB", 100)],
            "ADD:BNB",
        )
        tx2 = Transaction(
            Binance.chain,
            "USER",
            "VAULT",
            [Coin("BNB.BNB", 100)],
            "ADD:BNB",
        )
        self.assertEqual(tx1, tx2)
        tx1.memo = "STAKE:BNB"
        tx2.memo = "ADD:BNB"
        self.assertNotEqual(tx1, tx2)
        tx1.memo = "STAKE"
        tx2.memo = "ADD"
        self.assertNotEqual(tx1, tx2)
        tx1.memo = ""
        tx2.memo = ""
        self.assertEqual(tx1, tx2)
        tx1.memo = "Hello"
        tx2.memo = ""
        self.assertNotEqual(tx1, tx2)
        # we ignore addresses in memo
        tx1.memo = "REFUND:ADDRESS"
        tx2.memo = "REFUND:TODO"
        self.assertNotEqual(tx1, tx2)
        # we dont ignore different assets though
        tx1.memo = "ADD:BNB"
        tx2.memo = "ADD:RUNE"
        self.assertNotEqual(tx1, tx2)
        tx2.memo = "ADD:BNB"
        self.assertEqual(tx1, tx2)
        tx2.coins = [Coin("BNB.BNB", 100)]
        self.assertEqual(tx1, tx2)
        tx2.coins = [Coin("BNB.BNB", 100), Coin("RUNE", 100)]
        self.assertNotEqual(tx1, tx2)
        # different list of coins not equal
        tx1.coins = [Coin("RUNE", 200), Coin("RUNE", 100)]
        tx2.coins = [Coin("BNB.BNB", 100), Coin("RUNE", 200)]
        self.assertNotEqual(tx1, tx2)
        # coins different order tx are still equal
        tx1.coins = [Coin("RUNE", 200), Coin("BNB.BNB", 100)]
        tx2.coins = [Coin("BNB.BNB", 100), Coin("RUNE", 200)]
        self.assertEqual(tx1, tx2)
        # we ignore from / to address for equality
        tx1.to_address = "VAULT1"
        tx2.to_address = "VAULT2"
        tx1.from_address = "USER1"
        tx2.from_address = "USER2"
        self.assertNotEqual(tx1, tx2)
        # check list of transactions equality
        tx1 = Transaction(
            Binance.chain,
            "USER",
            "VAULT",
            [Coin("BNB.BNB", 100)],
            "ADD:BNB",
        )
        tx2 = deepcopy(tx1)
        tx3 = deepcopy(tx1)
        tx4 = deepcopy(tx1)
        list1 = [tx1, tx2]
        list2 = [tx3, tx4]
        self.assertEqual(list1, list2)

        # check sort list of transactions get sorted by smallest coin
        # check list of 1 coin
        # descending order in list1
        tx1.coins = [Coin("RUNE", 200)]
        tx2.coins = [Coin("BNB.BNB", 100)]
        # ascrending order in list2
        tx3.coins = [Coin("BNB.BNB", 100)]
        tx4.coins = [Coin("RUNE", 200)]
        self.assertNotEqual(list1, list2)
        self.assertEqual(sorted(list1), list2)
        self.assertEqual(sorted(list1), sorted(list2))

        # check list of > 1 coin
        # descending order in list1
        tx1.coins = [Coin("RUNE", 200), Coin("BNB.BNB", 300)]
        tx2.coins = [Coin("BNB.BNB", 100), Coin("LOK-3C0", 500)]
        # ascrending order in list2
        tx3.coins = [Coin("BNB.BNB", 100), Coin("LOK-3C0", 500)]
        tx4.coins = [Coin("RUNE", 200), Coin("BNB.BNB", 300)]
        self.assertNotEqual(list1, list2)
        self.assertEqual(sorted(list1), list2)
        self.assertEqual(sorted(list1), sorted(list2))

        # check 1 tx with no coins
        list1 = sorted(list1)
        self.assertEqual(list1, list2)
        list1[0].coins = None
        self.assertNotEqual(list1, list2)
        list2[0].coins = None
        self.assertEqual(list1, list2)

    def test_custom_hash(self):
        txn = Transaction(
            Binance.chain,
            "USER",
            "tbnb1yxfyeda8pnlxlmx0z3cwx74w9xevspwdpzdxpj",
            Coin("BNB.BNB", 194765912),
            "REFUND:9999A5A08D8FCF942E1AAAA01AB1E521B699BA3A009FA0591C011DC1FFDC5E68",
            id="9999A5A08D8FCF942E1AAAA01AB1E521B699BA3A009FA0591C011DC1FFDC5E68",
        )
        self.assertEqual(
            txn.custom_hash(""),
            "19B8CB9B42B455F3F239CFD5017D1BCF6D193FA41F5E3F03B4762F2290469F4C",
        )
        txn.coins = None
        self.assertEqual(
            txn.custom_hash(""),
            "3F9B8A2396D1D838CB37E26165AB6430590C97F5936268618004CA1545DFAEF8",
        )
        pubkey = "thorpub1addwnpepqv7kdf473gc4jyls7hlx4rg"
        self.assertEqual(
            txn.custom_hash(pubkey),
            "3B2624E03C79DEC01E16303143DFC145DD4870F9AE522926F6694A8FAF5C948C",
        )
        if DEFAULT_RUNE_ASSET == RUNE:
            self.assertEqual(
                txn.custom_hash(pubkey),
                "3B2624E03C79DEC01E16303143DFC145DD4870F9AE522926F6694A8FAF5C948C",
            )

    def test_to_json(self):
        txn = Transaction(
            Binance.chain,
            "USER",
            "VAULT",
            Coin("BNB.BNB", 100),
            "ADD:BNB",
        )
        self.assertEqual(
            txn.to_json(),
            '{"id": "TODO", "chain": "BNB", "from_address": "USER", '
            '"to_address": "VAULT", "memo": "ADD:BNB", "coins": '
            '[{"asset": "BNB.BNB", "amount": 100}], "gas": null, '
            '"max_gas": null, "fee": null}',
        )
        txn.coins = [Coin("BNB.BNB", 1000000000), Coin(RUNE, 1000000000)]
        self.assertEqual(
            txn.to_json(),
            '{"id": "TODO", "chain": "BNB", "from_address": "USER", '
            '"to_address": "VAULT", "memo": "ADD:BNB", "coins": ['
            '{"asset": "BNB.BNB", "amount": 1000000000}, '
            '{"asset": "'
            + RUNE
            + '", "amount": 1000000000}], "gas": null, "max_gas": null, '
            '"fee": null}',
        )
        txn.coins = None
        self.assertEqual(
            txn.to_json(),
            '{"id": "TODO", "chain": "BNB", "from_address": "USER", '
            '"to_address": "VAULT", "memo": "ADD:BNB", "coins": null, '
            '"gas": null, "max_gas": null, "fee": null}',
        )
        txn.gas = [Coin("BNB.BNB", 37500)]
        self.assertEqual(
            txn.to_json(),
            '{"id": "TODO", "chain": "BNB", "from_address": "USER", '
            '"to_address": "VAULT", "memo": "ADD:BNB", "coins": null,'
            ' "gas": [{"asset": "BNB.BNB", "amount": 37500}], '
            '"max_gas": null, "fee": null}',
        )

    def test_from_data(self):
        value = {
            "chain": "BNB",
            "from_address": "USER",
            "to_address": "VAULT",
            "coins": [
                {"asset": "BNB.BNB", "amount": 1000},
                {"asset": RUNE, "amount": "1000"},
            ],
            "memo": "ADD:BNB.BNB",
        }
        txn = Transaction.from_data(value)
        self.assertEqual(txn.chain, "BNB")
        self.assertEqual(txn.from_address, "USER")
        self.assertEqual(txn.to_address, "VAULT")
        self.assertEqual(txn.memo, "ADD:BNB.BNB")
        self.assertEqual(txn.coins[0].asset, "BNB.BNB")
        self.assertEqual(txn.coins[0].amount, 1000)
        self.assertEqual(txn.coins[1].asset, RUNE)
        self.assertEqual(txn.coins[1].amount, 1000)
        self.assertEqual(txn.gas, None)
        value["coins"] = None
        value["gas"] = [{"asset": "BNB.BNB", "amount": "37500"}]
        txn = Transaction.from_data(value)
        self.assertEqual(txn.chain, "BNB")
        self.assertEqual(txn.from_address, "USER")
        self.assertEqual(txn.to_address, "VAULT")
        self.assertEqual(txn.memo, "ADD:BNB.BNB")
        self.assertEqual(txn.coins, None)
        self.assertEqual(txn.gas[0].asset, "BNB.BNB")
        self.assertEqual(txn.gas[0].amount, 37500)


if __name__ == "__main__":
    unittest.main()
