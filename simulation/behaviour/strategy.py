# SPDX-License-Identifier: AGPL-3.0-or-later
# Copyright 2021-2025 The DFaaS Authors. All rights reserved.
# This file is licensed under the AGPL v3.0 or later license. See LICENSE and
# AUTHORS file for more information.

from __future__ import annotations
from abc import ABC, abstractmethod

class Strategy(ABC):
    @abstractmethod
    def run(self) -> dict:
        pass
