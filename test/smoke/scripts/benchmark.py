import argparse
import logging
import os
import time
import sys
from tqdm import tqdm

from chains.binance import MockBinance
from thorchain.thorchain import ThorchainState, ThorchainClient
from utils.common import Transaction, Coin, get_rune_asset
from chains.aliases import get_alias

# Init logging
logging.basicConfig(
    format="%(levelname).1s[%(asctime)s] %(message)s",
    level=os.environ.get("LOGLEVEL", "INFO"),
)

RUNE = get_rune_asset()


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--binance", default="http://localhost:26660", help="Mock binance server"
    )
    parser.add_argument(
        "--thorchain", default="http://localhost:1317", help="Thorchain API url"
    )
    parser.add_argument(
        "--thorchain-websocket",
        default="ws://localhost:26657/websocket",
        help="Thorchain Websocket url",
    )
    parser.add_argument(
        "--tx-type",
        default="swap",
        help="Transactions type to perform (swap or provide liquidity)",
    )
    parser.add_argument(
        "--num", type=int, default=100, help="Number of transactions to perform"
    )
    args = parser.parse_args()

    benchie = Benchie(
        args.binance, args.thorchain, args.tx_type, args.num, args.thorchain_websocket
    )
    try:
        benchie.run()
    except Exception as e:
        logging.fatal(e)
        sys.exit(1)


class Benchie:
    def __init__(self, bnb, thor, tx_type, num, thor_ws=None):
        self.thorchain = ThorchainState()

        self.thorchain_client = ThorchainClient(thor, thor_ws)
        vault_address = self.thorchain_client.get_vault_address("BNB")
        vault_pubkey = self.thorchain_client.get_vault_pubkey()

        self.thorchain.set_vault_pubkey(vault_pubkey)

        self.mock_binance = MockBinance(bnb)
        self.mock_binance.set_vault_address(vault_address)

        self.num = num
        self.tx_type = tx_type
        if self.tx_type != "swap" and self.tx_type != "add":
            logging.error("invalid tx type: " + self.tx_type)
            os.exit(1)

        time.sleep(5)  # give thorchain extra time to start the blockchain

    def error(self, err):
        self.exit = 1
        if self.fast_fail:
            raise Exception(err)
        else:
            logging.error(err)

    def run(self):
        logging.info(f">>> Starting benchmark... ({self.tx_type}: {self.num})")
        logging.info(">>> setting up...")
        # seed liquidity provider
        self.mock_binance.transfer(
            Transaction(
                "BNB",
                get_alias("BNB", "MASTER"),
                get_alias("BNB", "PROVIDER-1"),
                [
                    Coin("BNB.BNB", self.num * 100 * Coin.ONE),
                    Coin(RUNE, self.num * 100 * Coin.ONE),
                ],
            )
        )

        # seed swapper
        self.mock_binance.transfer(
            Transaction(
                "BNB",
                get_alias("BNB", "MASTER"),
                get_alias("BNB", "USER-1"),
                [
                    Coin("BNB.BNB", self.num * 100 * Coin.ONE),
                    Coin(RUNE, self.num * 100 * Coin.ONE),
                ],
            )
        )

        if self.tx_type == "swap":
            # provide BNB
            self.mock_binance.transfer(
                Transaction(
                    "BNB",
                    get_alias("BNB", "PROVIDER-1"),
                    get_alias("BNB", "VAULT"),
                    [
                        Coin("BNB.BNB", self.num * 100 * Coin.ONE),
                        Coin(RUNE, self.num * 100 * Coin.ONE),
                    ],
                    memo="ADD:BNB.BNB",
                )
            )

        time.sleep(5)  # give thorchain extra time to start the blockchain

        logging.info("<<< done.")
        logging.info(">>> compiling transactions...")
        txns = []
        memo = f"{self.tx_type}:BNB.BNB"
        for x in range(0, self.num):
            if self.tx_type == "add":
                coins = [
                    Coin(RUNE, 10 * Coin.ONE),
                    Coin("BNB.BNB", 10 * Coin.ONE),
                ]
            elif self.tx_type == "swap":
                coins = [
                    Coin(RUNE, 10 * Coin.ONE),
                ]
            txns.append(
                Transaction(
                    "BNB",
                    get_alias("BNB", "USER-1"),
                    get_alias("BNB", "VAULT"),
                    coins,
                    memo=memo,
                )
            )

        logging.info("<<< done.")
        logging.info(">>> broadcasting transactions...")
        self.mock_binance.transfer(txns)
        logging.info("<<< done.")

        logging.info(">>> timing for thorchain...")
        start_block_height = self.thorchain_client.get_block_height()
        t1 = time.time()
        completed = 0

        pbar = tqdm(total=self.num)
        while completed < self.num:
            events = self.thorchain_client.events
            if len(events) == 0:
                time.sleep(1)
                continue
            completed = len([e for e in events if e.type == self.tx_type.lower()])
            pbar.update(completed)
            time.sleep(1)
        pbar.close()

        t2 = time.time()
        end_block_height = self.thorchain_client.get_block_height()
        total_time = t2 - t1
        total_blocks = end_block_height - start_block_height
        logging.info("<<< done.")
        logging.info(f"({self.tx_type}: {completed}")
        logging.info(f"Blocks: {total_blocks}, {total_time} seconds)")


if __name__ == "__main__":
    main()
