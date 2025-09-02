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

-- Add the tenants column to the tables:
alter table cluster_templates add column tenants text[] not null default array['shared'];
alter table clusters add column tenants text[] not null default array['shared'];
alter table host_classes add column tenants text[] not null default array['shared'];
alter table hubs add column tenants text[] not null default array['shared'];

-- Add indexes on the tenants column:
create index cluster_templates_by_tenant on cluster_templates using gin (tenants);
create index clusters_by_tenant on clusters using gin (tenants);
create index host_classes_by_tenant on host_classes using gin (tenants);
create index hubs_by_tenant on hubs using gin (tenants);

-- Add the tenants column to the archive tables:
alter table archived_cluster_templates add column tenants text[] not null default array['shared'];
alter table archived_clusters add column tenants text[] not null default array['shared'];
alter table archived_host_classes add column tenants text[] not null default array['shared'];
alter table archived_hubs add column tenants text[] not null default array['shared'];
