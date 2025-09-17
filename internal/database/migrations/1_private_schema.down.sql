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

drop trigger if exists create_empty_private_cluster on clusters;
drop trigger if exists create_empty_private_cluster_order on cluster_orders;
drop function if exists private.create_empty_cluster();
drop function if exists private.create_empty_cluster_order();
drop schema if exists private cascade;