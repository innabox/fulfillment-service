--
-- Copyright (c) 2025 Red Hat Inc.
--
-- Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
-- the License. You may obtain a copy of the License at
--
--   http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on
-- an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
-- specific language governing permissions and limitations under the License.
--

-- Drop indexes on creators column
drop index if exists virtual_machines_by_owner;
drop index if exists virtual_machine_templates_by_owner;
drop index if exists hubs_by_owner;
drop index if exists host_classes_by_owner;
drop index if exists clusters_by_owner;
drop index if exists cluster_templates_by_owner;

-- Remove creators column from archive tables
alter table archived_virtual_machines drop column if exists creators;
alter table archived_virtual_machine_templates drop column if exists creators;
alter table archived_hubs drop column if exists creators;
alter table archived_host_classes drop column if exists creators;
alter table archived_clusters drop column if exists creators;
alter table archived_cluster_templates drop column if exists creators;

-- Remove creators column from main tables
alter table virtual_machines drop column if exists creators;
alter table virtual_machine_templates drop column if exists creators;
alter table hubs drop column if exists creators;
alter table host_classes drop column if exists creators;
alter table clusters drop column if exists creators;
alter table cluster_templates drop column if exists creators;