/*
 * Copyright (c) 2026 404Setup. All rights reserved.
 *
 * This Source Code Form is subject to the terms of the Mozilla Public License, v. 2.0. If a copy of the MPL was not distributed with this file, You can obtain one at https://mozilla.org/MPL/2.0/.
 *
 * If it is not possible or desirable to put the notice in a particular file, then You may include the notice in a location (such as a LICENSE file in a relevant directory) where a recipient would be likely to look for such a notice.
 *
 * This Source Code Form is "Incompatible With Secondary Licenses", as defined by the Mozilla Public License, v. 2.0.
 */

package core

// TrafficQuotaUserOverride stores administrator overrides for a specific user's traffic quota.
type TrafficQuotaUserOverride struct {
	Username            string
	HourlyLimitBytes    int64 // -1 if not overridden
	DailyLimitBytes     int64 // -1 if not overridden
	MonthlyLimitBytes   int64 // -1 if not overridden
	HourlyEnabled       int   // -1 if not overridden, 0 = false, 1 = true
	DailyEnabled        int   // -1 if not overridden, 0 = false, 1 = true
	MonthlyEnabled      int   // -1 if not overridden, 0 = false, 1 = true
	RecoveryAmountBytes int64 // -1 if not overridden
	RecoveryIntervalS   int64 // -1 if not overridden
	Unlimited           bool
	UpdatedAt           int64
}
