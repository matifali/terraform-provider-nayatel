// Copyright (c) 2026 Muhammad Atif Ali
// SPDX-License-Identifier: MPL-2.0

package client

import (
	"context"
	"fmt"
	"time"
)

// pollConfig configures pollUntil. Each WaitForStatus implementation has its
// own polling cadence and error tolerance (confirmed live: cubes return 403
// while provisioning and must keep polling through it; volumes have no
// working get-by-ID endpoint and a miss means the volume is gone, not still
// provisioning), so these are explicit knobs rather than one-size-fits-all
// behavior.
type pollConfig[T any] struct {
	interval time.Duration
	timeout  time.Duration
	fetch    func(context.Context) (*T, error)
	// tolerateFetchErr keeps polling on a fetch error instead of failing
	// immediately (needed while an item is provisioning and transiently
	// unreachable).
	tolerateFetchErr bool
	// notFoundIsFatal fails immediately when fetch returns (nil, nil)
	// instead of continuing to poll (used where a miss means the item
	// will never appear, rather than "not visible yet").
	notFoundIsFatal bool
	done            func(*T) bool
	failed          func(*T) bool
	timeoutMsg      string
	notFoundMsg     string
	failedMsg       string
}

// pollUntil polls cfg.fetch on cfg.interval until cfg.done reports the item
// has reached its target state, cfg.failed reports a terminal failure, or
// cfg.timeout elapses.
func pollUntil[T any](ctx context.Context, cfg pollConfig[T]) (*T, error) {
	ticker := time.NewTicker(cfg.interval)
	defer ticker.Stop()

	timeoutCh := time.After(cfg.timeout)

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-timeoutCh:
			return nil, fmt.Errorf("%s", cfg.timeoutMsg)
		case <-ticker.C:
			item, err := cfg.fetch(ctx)
			if err != nil {
				if cfg.tolerateFetchErr {
					continue
				}
				return nil, err
			}
			if item == nil {
				if cfg.notFoundIsFatal {
					return nil, fmt.Errorf("%s", cfg.notFoundMsg)
				}
				continue
			}
			if cfg.failed(item) {
				return nil, fmt.Errorf("%s", cfg.failedMsg)
			}
			if cfg.done(item) {
				return item, nil
			}
		}
	}
}
