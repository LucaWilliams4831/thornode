from chains.account import Account


class GenericChain:
    """
    A local simple implementation of a generic chain
    """

    name = "Generic Chain"
    chain = "RENAME_ME_CHAIN"
    coin = "RENAME_ME_COIN"

    def __init__(self):
        self.accounts = {}

    @classmethod
    def _calculate_gas(cls, pool, txn):
        """
        With given coin set, calculates the gas owed
        """
        raise Exception("_calculate_gas is not yet implemented")

    def get_account(self, addr):
        """
        Retrieve an accout by address
        """
        if addr in self.accounts:
            return self.accounts[addr]
        return Account(addr)

    def set_account(self, acct):
        """
        Update a given account
        """
        self.accounts[acct.address] = acct

    def transfer(self, txn):
        """
        Makes a transfer on the generic chain. Returns gas used
        """

        if txn.chain != self.chain:
            raise Exception(f"Cannot transfer. {self.chain} is not {txn.chain}")

        from_acct = self.get_account(txn.from_address)
        to_acct = self.get_account(txn.to_address)

        if not txn.gas:
            txn.gas = [self._calculate_gas(None, txn)]

        from_acct.sub(txn.gas[0])

        from_acct.sub(txn.coins)
        to_acct.add(txn.coins)

        self.set_account(from_acct)
        self.set_account(to_acct)
