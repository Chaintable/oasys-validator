// Copyright 2026 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rawdb

import (
	"testing"

	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/params"
)

func TestBlockHistorySetsFreezerThreshold(t *testing.T) {
	tests := []struct {
		name         string
		blockHistory uint64
		want         uint64
	}{
		{name: "disabled", blockHistory: 0, want: params.FullImmutabilityThreshold},
		{name: "below default", blockHistory: 14_000, want: 14_000},
		{name: "at default", blockHistory: params.FullImmutabilityThreshold, want: params.FullImmutabilityThreshold},
		{name: "above default", blockHistory: params.FullImmutabilityThreshold + 1, want: params.FullImmutabilityThreshold},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			freezer, err := newChainFreezer("", "", "", false)
			if err != nil {
				t.Fatalf("failed to create chain freezer: %v", err)
			}
			defer freezer.Close()

			if err := freezer.SetupFreezerEnv(&ethdb.FreezerEnv{}, test.blockHistory); err != nil {
				t.Fatalf("failed to set freezer environment: %v", err)
			}
			if got := freezer.threshold.Load(); got != test.want {
				t.Fatalf("unexpected freezer threshold: got %d, want %d", got, test.want)
			}
			if got := freezer.blockHistory.Load(); got != test.blockHistory {
				t.Fatalf("unexpected block history: got %d, want %d", got, test.blockHistory)
			}
		})
	}
}
