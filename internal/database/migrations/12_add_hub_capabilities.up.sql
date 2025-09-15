--
-- Copyright (c) 2025 Red Hat Inc.
--
-- Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with
-- the License. You may obtain a copy of the License at
--
--   http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the
-- specific language governing permissions and limitations under the License.
--

-- Add capabilities and hub_type columns to hubs table
alter table hubs add column capabilities text[] not null default array['clusters'];

-- Add hub_type column with default value of 'cluster' for backward compatibility
alter table hubs add column hub_type text not null default 'cluster';

-- Create GIN index for efficient capabilities array queries
create index hubs_by_capability on hubs using gin (capabilities);

-- Create regular index for hub_type queries
create index hubs_by_type on hubs (hub_type);

-- Update any existing hubs to have default capabilities
-- This ensures backward compatibility for existing deployments
update hubs set capabilities = array['clusters'], hub_type = 'cluster' where capabilities = array[]::text[];