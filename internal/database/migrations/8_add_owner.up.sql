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

-- Add the owner column to the tables:
alter table cluster_templates add column owner text not null default 'unknown';
alter table clusters add column owner text not null default 'unknown';
alter table host_classes add column owner text not null default 'unknown';
alter table hubs add column owner text not null default 'unknown';

-- Add indexes on the owner column:
create index cluster_templates_by_owner on cluster_templates (owner);
create index clusters_by_owner on clusters (owner);
create index host_classes_by_owner on host_classes (owner);
create index hubs_by_owner on hubs (owner);

-- Add the owner column to the archive tables:
alter table archived_cluster_templates add column owner text not null default 'unknown';
alter table archived_clusters add column owner text not null default 'unknown';
alter table archived_host_classes add column owner text not null default 'unknown';
alter table archived_hubs add column owner text not null default 'unknown';
