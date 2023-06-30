"""Bank module message types."""

from __future__ import annotations

from typing import Any

from bech32 import bech32_decode, convertbits

from thornode_proto.types import MsgDeposit as MsgDeposit_pb

from utils.common import Coins
from terra_sdk.core.msg import Msg

__all__ = ["MsgDeposit"]

import attr


@attr.s
class MsgDeposit(Msg):
    """Deposit native assets on thorchain from ``signer`` to
     asgard module with ``coins`` and ``memo``.
    Args:
        coins (Coins): coins to deposit
        memo: memo
        signer (Coins): signer
    """

    type_amino = "thorchain/MsgDeposit"
    """"""
    type_url = "/types.MsgDeposit"
    """"""
    action = "deposit"
    """"""

    coins: Coins = attr.ib(converter=Coins)
    memo: str = attr.ib()
    signer: str = attr.ib()

    @classmethod
    def from_data(cls, data: dict) -> MsgDeposit:
        return cls(
            coins=Coins.from_data(data["coins"]),
            memo=data["memo"],
            signer=data["signer"],
        )

    def to_data(self) -> dict:
        return {
            "@type": self.type_url,
            "coins": self.coins.to_data(),
            "memo": self.memo,
            "signer": self.signer,
        }

    @classmethod
    def from_proto(cls, proto: MsgDeposit_pb) -> MsgDeposit:
        return cls(
            coins=Coins.from_proto(proto["coins"]),
            memo=proto["memo"],
            signer=proto["signer"],
        )

    def to_proto(self) -> MsgDeposit_pb:
        data = bech32_decode(self.signer)[1]
        signer = convertbits(data, 5, 8, False)
        proto = MsgDeposit_pb()
        proto.coins = self.coins.to_proto()
        proto.memo = self.memo
        proto.signer = bytes(signer)
        return proto

    @classmethod
    def unpack_any(cls, any: Any) -> MsgDeposit:
        return MsgDeposit.from_proto(any)
