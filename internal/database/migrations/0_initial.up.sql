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

create table cluster_templates (
  id uuid not null primary key,
  title text not null,
  description text not null
);

create table clusters (
  id uuid not null primary key,
  api_url text not null,
  console_url text not null
);

create type cluster_order_state as enum (
  'UNSPECIFIED',
  'ACCEPTED',
  'REJECTED',
  'CANCELED',
  'FULFILLED',
  'FAILED'
);

create table cluster_orders (
  id uuid not null primary key,
  template_id uuid not null,
  state cluster_order_state not null default 'UNSPECIFIED',
  cluster_id uuid,
  constraint fk_template foreign key (template_id) references cluster_templates (id),
  constraint fk_cluster foreign key (cluster_id) references clusters (id)
);
