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

create table changes (
  serial serial not null primary key,
  timestamp timestamp with time zone not null default now(),
  source text,
  operation text,
  old jsonb,
  new jsonb
);

create function save_changes() returns trigger as $$
begin
  insert into changes (
    source,
    operation,
    old,
    new
  ) values (
    tg_table_name,
    lower(tg_op),
    row_to_json(old.*),
    row_to_json(new.*)
  );
  notify changes;
  return null;
end;
$$ language plpgsql;

create trigger save_cluster_changes
after insert or update or delete on clusters
for each row execute function save_changes();

create trigger save_cluster_order_changes
after insert or update or delete on cluster_orders
for each row execute function save_changes();

create trigger save_cluster_template_changes
after insert or update or delete on cluster_templates
for each row execute function save_changes();
