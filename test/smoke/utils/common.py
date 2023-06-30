import requests
import logging
import json
import os
import hashlib

from decimal import Decimal, getcontext

from thornode_proto.common import Coin as Coin_pb
from thornode_proto.common import Asset as Asset_pb

from requests.adapters import HTTPAdapter
from requests.packages.urllib3.util.retry import Retry

DEFAULT_RUNE_ASSET = "THOR.RUNE"


def get_rune_asset():
    return Asset(os.environ.get("RUNE", DEFAULT_RUNE_ASSET))


def requests_retry_session(
    retries=6,
    backoff_factor=1,
    status_forcelist=(500, 502, 504),
    session=None,
):
    """
    Creates a request session that has auto retry
    """
    session = session or requests.Session()
    retry = Retry(
        total=retries,
        read=retries,
        connect=retries,
        backoff_factor=backoff_factor,
        status_forcelist=status_forcelist,
    )
    adapter = HTTPAdapter(max_retries=retry)
    session.mount("http://", adapter)
    session.mount("https://", adapter)
    return session


def get_share(part, total, alloc):
    """
    Calculates the share of something
    (Allocation / (Total / part))
    """
    if total == 0 or part == 0:
        return 0
    getcontext().prec = 18
    return int(round(Decimal(alloc) / (Decimal(total) / Decimal(part))))


def get_diff(current, previous):
    if current == previous:
        return 0
    try:
        return (abs(current - previous) / ((current + previous) / 2)) * 100.0
    except ZeroDivisionError:
        return float("inf")


class HttpClient:
    """
    An generic http client
    """

    def __init__(self, base_url):
        self.base_url = base_url

    def get_url(self, path):
        """
        Get fully qualified url with given path
        """
        return self.base_url + path

    def fetch(self, path, args={}):
        """
        Make a get request
        """
        url = self.get_url(path)
        resp = requests_retry_session().get(url, params=args)
        resp.raise_for_status()
        return resp.json()

    def fetch_plain(self, path, args={}):
        """
        Make a get request , return the plain response
        """
        url = self.get_url(path)
        resp = requests_retry_session().get(url, params=args)
        resp.raise_for_status()
        return resp.text

    def post(self, path, payload={}):
        """
        Make a post request
        """
        url = self.get_url(path)
        resp = requests_retry_session().post(url, json=payload)
        if resp.status_code != 200:
            logging.error(resp.text)
        resp.raise_for_status()
        return json.loads(resp.text, parse_float=Decimal)


class Jsonable:
    def to_json(self):
        return json.dumps(self, default=lambda x: x.__dict__)


class Asset(str, Jsonable):
    def __init__(self, value):
        if "/" in value:
            self.is_synth = True
            parts = value.split("/")
        else:
            self.is_synth = False
            parts = value.split(".")

        if len(parts) == 1:
            self.chain = "THOR"
            self.symbol = parts[0]
        else:
            self.chain = parts[0]
            self.symbol = parts[1]

    def is_bnb(self):
        """
        Is this asset bnb?
        """
        return self.get_symbol().startswith("BNB")

    def is_btc(self):
        """
        Is this asset btc?
        """
        return self.get_symbol().startswith("BTC")

    def is_bch(self):
        """
        Is this asset bch?
        """
        return self.get_symbol().startswith("BCH")

    def is_ltc(self):
        """
        Is this asset ltc?
        """
        return self.get_symbol().startswith("LTC")

    def is_doge(self):
        """
        Is this asset doge?
        """
        return self.get_symbol().startswith("DOGE")

    def is_eth(self):
        """
        Is this asset eth?
        """
        return self.get_symbol().startswith("ETH")

    def is_luna(self):
        """
        Is this asset luna?
        """
        return self.get_symbol().startswith("LUNA")

    def is_gaia(self):
        """
        Is this asset gaia chain?
        """
        return self.get_chain() == "GAIA"

    def is_thor(self):
        """
        Is this asset thorchain chain?
        """
        return self.get_chain() == "THOR"

    def is_erc(self):
        """
        Is this asset erc20?
        """
        return self.get_chain() == "ETH" and not self.get_symbol().startswith("ETH")

    def is_rune(self):
        """
        Is this asset rune?
        """
        return self.get_symbol().startswith("RUNE")

    def get_symbol(self):
        """
        Return symbol part of the asset string
        """
        return self.symbol

    def get_ticker(self):
        """
        Return ticker part of the asset
        """
        return self.symbol.split("-")[0]

    def get_chain(self):
        """
        Return chain part of the asset string
        """
        if self.is_synth:
            return "THOR"
        return self.chain

    def get_synth_asset(self):
        """
        Return synth asset
        """
        if self.is_synth:
            return self
        return Asset(str(self).replace(".", "/"))

    def get_layer1_asset(self):
        """
        Return layer1 asset
        """
        if not self.is_synth:
            return self
        return Asset(str(self).replace("/", "."))
    
    def is_synth_asset(self):
        """
        Return if asset is synth
        """
        return self.is_synth

    def __eq__(self, other):
        if isinstance(other, str):
            other = Asset(other)
        return self.get_chain() == other.get_chain() and self.get_symbol() == other.get_symbol()

    def __str__(self):
        div = "."
        if self.is_synth:
            div = "/"
        return f"{self.chain}{div}{self.get_symbol()}"

    def __hash__(self):
        return hash(str(self))

    @classmethod
    def from_data(cls, value):
        if value["is_synth"]:
            return cls(f"{value['chain']}/{value['symbol']}")
        return cls(f"{value['chain']}.{value['symbol']}")

    @classmethod
    def from_proto(cls, proto):
        if proto.synth:
            return cls(f"{proto.chain}/{proto.symbol}")
        return cls(f"{proto.chain}.{proto.symbol}")

    def to_proto(self):
        asset = Asset_pb()
        asset.chain = self.chain
        asset.symbol = self.symbol
        asset.synth = self.is_synth
        asset.ticker = self.get_ticker()
        return asset


class Coin(Jsonable):
    """
    A class that represents a coin and an amount
    """

    ONE = 100000000

    def __init__(self, asset, amount=0):
        if isinstance(asset, Asset):
            self.asset = asset
        else:
            self.asset = Asset(asset)
        self.amount = int(amount)

    def is_rune(self):
        """
        Is this coin rune?
        """
        return self.asset.is_rune()

    def is_zero(self):
        """
        Is the amount of this coin zero?
        """
        return self.amount == 0

    def to_thorchain_fmt(self):
        """
        Convert the class to an dictionary, specifically in a format for
        thorchain
        """
        return {
            "asset": self.asset,
            "amount": str(self.amount),
        }

    def to_binance_fmt(self):
        """
        Convert the class to an dictionary, specifically in a format for
        mock-binance
        """
        return {
            "denom": self.asset.get_symbol(),
            "amount": self.amount,
        }

    def to_cosmos_gaia(self):
        amount = int(self.amount / 100)
        return f"{amount}u{self.asset.get_symbol().lower()}"

    def to_cosmos_str(self):
        return f"{self.amount}u{self.asset.get_symbol().lower()}"

    def __eq__(self, other):
        return self.asset == other.asset and self.amount == other.amount

    def __lt__(self, other):
        return self.amount < other.amount

    def __sub__(self, other):
        return self.amount - other.amount

    def __add__(self, other):
        return self.amount + other.amount

    def __hash__(self):
        return hash(str(self))

    @classmethod
    def from_proto(cls, proto):
        return cls(Asset.from_proto(proto.asset), proto.amount)

    def to_proto(self):
        coin = Coin_pb()
        coin.asset = self.asset.to_proto()
        coin.amount = str(self.amount)
        return coin

    @classmethod
    def from_data(cls, value):
        return cls(str(value["asset"]), value["amount"])

    def to_data(self):
        """
        Convert the class to an dictionary
        """
        return {
            "asset": str(self.asset),
            "amount": self.amount,
        }

    def __repr__(self):
        return f"<Coin {self.amount/1e8:0,.8f} {self.asset}>"

    def __str__(self):
        return f"{self.amount/1e8:0,.8f} {self.asset}"

    def str_amt(self):
        return f"{self.amount/1e8:0,.8f}"


class Coins(Jsonable):
    def __init__(self, coins):
        self.coins = coins

    @classmethod
    def from_data(cls, data):
        """Converts list of Coin-data objects to :class:`Coins`.

        Args:
            data (list): list of Coin-data objects
        """
        coins = map(Coin.from_data, data)
        return cls(coins)

    def to_data(self):
        return [coin.to_data() for coin in self]

    @classmethod
    def from_proto(cls, proto):
        """Converts list of Coin-data objects to :class:`Coins`.

        Args:
            data (list): list of Coin-data objects
        """
        coins = map(Coin.from_proto, proto)
        return cls(coins)

    def to_proto(self):
        return [coin.to_proto() for coin in self]

    def __iter__(self):
        return iter(self.coins)


class Transaction(Jsonable):
    """
    A transaction on a block chain (ie Binance)
    """

    empty_id = "0000000000000000000000000000000000000000000000000000000000000000"

    def __init__(
        self,
        chain,
        from_address,
        to_address,
        coins,
        memo="",
        gas=None,
        id="TODO",
        max_gas=None,
    ):
        self.id = id.upper()
        self.chain = chain
        self.from_address = from_address
        self.to_address = to_address
        self.memo = memo

        # ensure coins is a list of coins
        if coins and not isinstance(coins, list):
            coins = [coins]
        self.coins = coins

        # ensure gas is a list of coins
        if gas and not isinstance(gas, list):
            gas = [gas]
        self.gas = gas
        if max_gas and not isinstance(max_gas, list):
            max_gas = [max_gas]
        self.max_gas = max_gas
        self.fee = None

    def __repr__(self):
        coins = self.coins if self.coins else "No Coins"
        gas = f" | Gas {self.gas}" if self.gas else ""
        fee = f" | Fee {self.fee}" if self.fee else ""
        id = f" | ID {self.id.upper()}" if self.id != "TODO" else ""
        return (
            f"<Tx {self.from_address:>10} => {self.to_address:10} "
            f"[{self.memo}] {coins}{gas}{fee}{id}>"
        )

    def __str__(self):
        coins = ", ".join([str(c) for c in self.coins]) if self.coins else "No Coins"
        gas = " | Gas " + ", ".join([str(g) for g in self.gas]) if self.gas else ""
        fee = f" | Fee {str(self.fee)}" if self.fee else ""
        id = (
            f" | ID {self.id.upper()}"
            if self.id != "TODO" and self.id != self.empty_id
            else ""
        )
        return (
            f"{self.from_address:>10} => {self.to_address:10} "
            f"[{self.memo}] {coins}{gas}{fee}{id}"
        )

    def short(self):
        coins = ", ".join([str(c) for c in self.coins]) if self.coins else "No Coins"
        gas = ", ".join([str(g) for g in self.gas]) if self.gas else ""
        fee = str(self.fee) if self.fee else ""
        return f"{coins} | Fee {fee} | Gas {gas}"

    def __eq__(self, other):
        """
        Check transaction equals another one
        Ignore from to address fields because our thorchain state
        doesn't know the "real" addresses yet
        """
        coins = self.coins or []
        other_coins = other.coins or []
        gas = self.gas or []
        other_gas = other.gas or []
        return (
            (
                self.id == "TODO"
                or self.id == self.empty_id
                or self.id.upper() == other.id.upper()
            )
            and self.chain == other.chain
            and self.memo == other.memo
            and self.from_address == other.from_address
            and self.to_address == other.to_address
            and sorted(coins) == sorted(other_coins)
            and sorted(gas) == sorted(other_gas)
        )

    def __lt__(self, other):
        coins = self.coins or []
        other_coins = other.coins or []
        return sorted(coins) < sorted(other_coins)

    def get_asset_from_memo(self):
        parts = self.memo.split(":")
        if len(parts) >= 2 and parts[1] != "":
            return Asset(parts[1])
        return None

    def is_cross_chain_provision(self):
        if not self.memo.startswith("ADD:"):
            return False
        if len(self.memo.split(":")) == 3:
            return True
        return False

    def is_synth(self):
        if "SYNTH" in self.memo:
            self.memo = self.memo.replace("-SYNTH", "")
            return True
        return False

    def is_refund(self):
        if "REFUND" in self.memo:
            return True
        return False

    def custom_hash(self, pubkey):
        coins = (
            ", ".join([f"{c.amount} {c.asset}" for c in self.coins])
            if self.coins
            else ""
        )
        in_hash = self.memo.split(":")[1]
        tmp = f"{self.chain}|{self.to_address}|{pubkey}|{coins}||{in_hash}"
        return hashlib.new("sha256", tmp.encode()).digest().hex().upper()

    def get_attributes(self):
        return [
            {"id": self.id},
            {"chain": self.chain},
            {"from": self.from_address},
            {"to": self.to_address},
            {"coin": self.coins_str()},
            {"memo": self.memo},
        ]

    def coins_str(self):
        return ", ".join([f"{c.amount} {c.asset}" for c in self.coins])

    @classmethod
    def from_data(cls, value):
        txn = cls(
            value["chain"],
            value["from_address"],
            value["to_address"],
            None,
            memo=value["memo"],
        )
        if "id" in value and value["id"]:
            txn.id = value["id"].upper()
        if "coins" in value and value["coins"]:
            txn.coins = [Coin.from_data(c) for c in value["coins"]]
        if "gas" in value and value["gas"]:
            txn.gas = [Coin.from_data(g) for g in value["gas"]]
        return txn

    @classmethod
    def empty_txn(cls):
        return Transaction("", "", "", None, id=cls.empty_id)
