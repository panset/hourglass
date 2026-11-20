-- SPDX-License-Identifier: MIT
-- D40: the core no longer derives resource names. A provider's allocation
-- belongs to the setup whose name equals its resource name, so the per-setup
-- override column and the stored resource-name template setting go away.
-- The setups table keeps every other column and all of its rows.
ALTER TABLE setups DROP COLUMN resource_name_override;
DELETE FROM settings WHERE key = 'resource_name_template';
