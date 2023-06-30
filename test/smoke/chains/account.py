import logging
from utils.common import Asset


class Account:
    """
    An account is an address with a list of coin balances associated
    """

    def __init__(self, address):
        self.address = address
        self.balances = []

    def sub(self, coins):
        """
        Subtract coins from balance
        """
        if not isinstance(coins, list):
            coins = [coins]
        for coin in coins:
            for i, c in enumerate(self.balances):
                if coin.asset == c.asset:
                    self.balances[i].amount -= coin.amount
                    if self.balances[i].amount < 0:
                        logging.info(f"Balance: {self.address} {self.balances[i]}")
                        self.balances[i].amount = 0
                        # raise Exception("insufficient funds")

    def add(self, coins):
        """
        Add coins to balance
        """
        if not isinstance(coins, list):
            coins = [coins]

        for coin in coins:
            found = False
            for i, c in enumerate(self.balances):
                if coin.asset == c.asset:
                    self.balances[i].amount += coin.amount
                    found = True
                    break
            if not found:
                self.balances.append(coin)

    def get(self, asset):
        """
        Get a specific coin by asset
        """
        if isinstance(asset, str):
            asset = Asset(asset)
        for coin in self.balances:
            if asset == coin.asset:
                return coin.amount
        return 0

    def __repr__(self):
        return "<Account %s: %s>" % (self.address, self.balances)

    def __str__(self):
        return "Account %s: %s" % (self.address, self.balances)
