import unittest
import os
import logging
import json
from pprint import pformat
from deepdiff import DeepDiff
from copy import deepcopy

from chains.binance import Binance
from chains.bitcoin import Bitcoin
from chains.litecoin import Litecoin
from chains.dogecoin import Dogecoin
from chains.gaia import Gaia
from chains.bitcoin_cash import BitcoinCash
from chains.ethereum import Ethereum
from thorchain.thorchain import ThorchainState, Event
from utils.breakpoint import Breakpoint
from utils.common import Transaction, get_rune_asset, DEFAULT_RUNE_ASSET

RUNE = get_rune_asset()
# Init logging
logging.basicConfig(
    format="%(levelname).1s[%(asctime)s] %(message)s",
    level=os.environ.get("LOGLEVEL", "INFO"),
)


def get_balance(idx):
    """
    Retrieve expected balance with given id
    """
    file = "data/smoke_test_balances.json"
    with open(file) as f:
        contents = f.read()
        contents = contents.replace(DEFAULT_RUNE_ASSET, RUNE)
        balances = json.loads(contents)
        for bal in balances:
            if idx == bal["TX"]:
                return bal


def get_events():
    """
    Retrieve expected events
    """
    file = "data/smoke_test_events.json"
    with open(file) as f:
        contents = f.read()
        contents = contents.replace(DEFAULT_RUNE_ASSET, RUNE)
        events = json.loads(contents)
        return [Event(e["type"], e["attributes"]) for e in events]
    raise Exception("could not load events")


class TestSmoke(unittest.TestCase):
    """
    This runs tests with a pre-determined list of transactions and an expected
    balance after each transaction (/data/balance.json). These transactions and
    balances were determined earlier via a google spreadsheet
    https://docs.google.com/spreadsheets/d/1sLK0FE-s6LInWijqKgxAzQk2RiSDZO1GL58kAD62ch0/edit#gid=439437407
    """

    def test_smoke(self):
        export = os.environ.get("EXPORT", None)
        export_events = os.environ.get("EXPORT_EVENTS", None)

        failure = False
        snaps = []
        bnb = Binance()  # init local binance chain
        btc = Bitcoin()  # init local bitcoin chain
        ltc = Litecoin()  # init local litecoin chain
        doge = Dogecoin()  # init local dogecoin chain
        gaia = Gaia()  # init local gaia chain
        bch = BitcoinCash()  # init local bitcoin cash chain
        eth = Ethereum()  # init local ethereum chain
        thorchain = ThorchainState()  # init local thorchain
        thorchain.network_fees = {  # init fixed network fees
            "BNB": 37500,
            "BTC": 10000,
            "LTC": 10000,
            "BCH": 10000,
            "DOGE": 10000,
            "GAIA": 20000,
            "ETH": 65000,
        }

        file = "data/smoke_test_transactions.json"

        with open(file, "r") as f:
            contents = f.read()
            loaded = json.loads(contents)

        for i, txn in enumerate(loaded):
            txn = Transaction.from_data(txn)
            logging.info(f"{i} {txn}")

            if txn.chain == Binance.chain:
                bnb.transfer(txn)  # send transfer on binance chain
            if txn.chain == Bitcoin.chain:
                btc.transfer(txn)  # send transfer on bitcoin chain
            if txn.chain == Litecoin.chain:
                ltc.transfer(txn)  # send transfer on litecoin chain
            if txn.chain == Dogecoin.chain:
                doge.transfer(txn)  # send transfer on dogecoin chain
            if txn.chain == Gaia.chain:
                gaia.transfer(txn)  # send transfer on gaia chain
            if txn.chain == BitcoinCash.chain:
                bch.transfer(txn)  # send transfer on bitcoin cash chain
            if txn.chain == Ethereum.chain:
                eth.transfer(txn)  # send transfer on ethereum chain
                # convert the coin amount to thorchain amount which is 1e8
                for idx, c in enumerate(txn.coins):
                    txn.coins[idx].amount = c.amount / 1e10
                for idx, c in enumerate(txn.gas):
                    txn.gas[idx].amount = c.amount / 1e10

            if txn.memo == "SEED":
                continue
            outbounds = thorchain.handle(txn)  # process transaction in thorchain

            for txn in outbounds:
                if txn.chain == Binance.chain:
                    bnb.transfer(txn)  # send outbound txns back to Binance
                if txn.chain == Bitcoin.chain:
                    btc.transfer(txn)  # send outbound txns back to Bitcoin
                if txn.chain == Litecoin.chain:
                    ltc.transfer(txn)  # send outbound txns back to Litecoin
                if txn.chain == Dogecoin.chain:
                    doge.transfer(txn)  # send outbound txns back to Dogecoin
                if txn.chain == Gaia.chain:
                    gaia.transfer(txn)  # send outbound txns back to Gaia
                if txn.chain == BitcoinCash.chain:
                    bch.transfer(txn)  # send outbound txns back to Bitcoin Cash
                if txn.chain == Ethereum.chain:
                    temp_txn = deepcopy(txn)
                    for idx, c in enumerate(temp_txn.coins):
                        temp_txn.coins[idx].amount = c.amount * 1e10
                    for idx, c in enumerate(temp_txn.gas):
                        temp_txn.gas[idx].amount = c.amount * 1e10
                    temp_txn.fee.amount = temp_txn.fee.amount * 1e10
                    eth.transfer(temp_txn)  # send outbound txns back to Ethereum

            thorchain.handle_rewards()

            bnb_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "BNB":
                    bnb_out.append(out)
            btc_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "BTC":
                    btc_out.append(out)
            ltc_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "LTC":
                    ltc_out.append(out)
            doge_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "DOGE":
                    doge_out.append(out)
            gaia_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "GAIA":
                    gaia_out.append(out)
            bch_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "BCH":
                    bch_out.append(out)
            eth_out = []
            for out in outbounds:
                if out.coins[0].asset.get_chain() == "ETH":
                    eth_out.append(out)
            thorchain.handle_gas(bnb_out)  # subtract gas from pool(s)
            thorchain.handle_gas(btc_out)  # subtract gas from pool(s)
            thorchain.handle_gas(ltc_out)  # subtract gas from pool(s)
            thorchain.handle_gas(doge_out)  # subtract gas from pool(s)
            thorchain.handle_gas(gaia_out)  # subtract gas from pool(s)
            thorchain.handle_gas(bch_out)  # subtract gas from pool(s)
            thorchain.handle_gas(eth_out)  # subtract gas from pool(s)

            # generated a snapshop picture of thorchain and bnb
            snap = Breakpoint(thorchain, bnb).snapshot(i, len(outbounds))
            snaps.append(snap)
            expected = get_balance(i)  # get the expected balance from json file

            diff = DeepDiff(
                snap, expected, ignore_order=True
            )  # empty dict if are equal
            if len(diff) > 0:
                logging.info(f"Transaction: {i} {txn}")
                logging.info(">>>>>> Expected")
                logging.info(pformat(expected))
                logging.info(">>>>>> Obtained")
                logging.info(pformat(snap))
                logging.info(">>>>>> DIFF")
                logging.info(pformat(diff))
                if not export:
                    raise Exception("did not match!")

            # log result
            if len(outbounds) == 0:
                continue
            result = "[+]"
            if "REFUND" in outbounds[0].memo:
                result = "[-]"
            for outbound in outbounds:
                logging.info(f"{result} {outbound.short()}")

        if export:
            with open(export, "w") as fp:
                json.dump(snaps, fp, indent=4)

        if export_events:
            with open(export_events, "w") as fp:
                json.dump(thorchain.events, fp, default=lambda x: x.__dict__, indent=4)

        # check events against expected
        expected_events = get_events()
        for event, expected_event in zip(thorchain.events, expected_events):
            if event != expected_event:
                logging.error(
                    f"Event Thorchain {event} \n   !="
                    f"  \nEvent Expected {expected_event}"
                )

                if not export_events:
                    raise Exception("Events mismatch")

        if failure:
            raise Exception("Fail")


if __name__ == "__main__":
    unittest.main()
