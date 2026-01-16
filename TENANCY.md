# Multi-Tenancy Design

The fulfillment service implements a sophisticated multi-tenancy architecture that provides secure
isolation between different tenants while maintaining high performance for data access operations.
This design ensures that users can only access resources that belong to tenants they have
permissions for, creating a robust foundation for multi-tenant applications.

## Architecture Overview

The multi-tenancy feature is built around a layered architecture that separates concerns between
authentication, authorization, and data access. At the heart of this system lies the `TenancyLogic`
interface, which serves as the primary abstraction for determining tenant relationships and
visibility rules.

The system integrates with external authentication and authorization services through
industry-standard protocols, specifically leveraging Envoy's external authentication extension.
This approach provides flexibility in choosing authentication providers while maintaining a
consistent internal interface for tenant management.

## External Authentication Integration

When a request arrives at the fulfillment service, it first passes through an Envoy proxy
configured with the external authentication filter. This proxy intercepts all incoming requests
and forwards authentication and authorization decisions to an external service before allowing the
request to proceed to the fulfillment service itself.

The service is currently configured to work with Authorino, a Kubernetes-native authorization
service that implements the Envoy external authentication protocol. However, the architecture is
designed to be provider-agnostic, meaning it can work with any external authentication service
that supports the Envoy external auth protocol, such as Open Policy Agent (OPA), Ory Oathkeeper,
or custom authentication services that implement the required gRPC interface.

The external authentication service performs several critical functions in the multi-tenancy
workflow. It validates the user's credentials, determines their identity and group memberships,
and makes authorization decisions about which operations the user is permitted to perform. Once
these decisions are made, the authentication service communicates this information to the
fulfillment service through standardized headers, particularly the `x-subject` header which
contains serialized user identity and tenant information.

## Tenant Assignment and Visibility

The `TenancyLogic` interface defines two fundamental operations that drive the multi-tenancy
behavior: determining assigned tenants for new objects and determining visible tenants for data
access operations.

When a user creates a new object in the system, the `DetermineAssignedTenants` method is called to
establish which tenants should be associated with that object. This method examines the
authentication context provided by the external authentication service and extracts relevant
tenant information from the user's identity, group memberships, or other attributes. The resulting
tenant list is then stored in the database as part of the object's metadata, specifically in a
`tenants` field that serves as the foundation for all subsequent access control decisions.

The tenant assignment logic can be sophisticated, taking into account various factors such as the
user's organizational membership, project assignments, or role-based rules. For example, a user
might be assigned to multiple tenants based on their participation in different projects or
departments, and objects they create could be assigned to one or more of these tenants depending
on the context of the creation request.

For data access operations, the `DetermineVisibleTenants` method calculates which tenants the
current user has permission to see. This determination is made independently for each request,
allowing for dynamic access control that can respond to changes in user permissions, group
memberships, or organizational structure without requiring updates to existing data.

## Data Access Control

The multi-tenancy system implements access control at the database level, ensuring that users can
only access objects belonging to tenants they have visibility permissions for. This approach
provides several important benefits, including efficient list operations and complete isolation
between tenants.

When a user attempts to retrieve, update, or delete an object, the system compares the tenants
visible to the user with the tenants stored in the object's metadata. Access is granted only when
there is a non-empty intersection between these two sets. This means that if a user has visibility
to any of the tenants assigned to an object, they can access that object.

The access control mechanism is designed to be transparent from the user's perspective. When a
user lacks permission to access an object, the system responds as if the object does not exist
rather than returning an explicit permission denied error. This approach prevents information
leakage about the existence of resources that users should not be aware of.

## Efficient List Operations

One of the key architectural decisions in the multi-tenancy design is the implementation of tenant
filtering at the database level using SQL queries. This approach enables efficient list operations
even in systems with large numbers of objects distributed across many tenants.

When a user requests a list of objects, the system automatically constructs SQL queries that
include tenant visibility filters. These filters use PostgreSQL's array operations to check for
intersections between the user's visible tenants and the tenants assigned to each object. The
database can leverage indexes on the tenants column to perform these operations efficiently, even
when dealing with millions of objects.

The SQL-based filtering approach contrasts with application-level filtering, which would require
retrieving all objects from the database and then filtering them in memory. By pushing the
filtering logic down to the database layer, the system can take advantage of database
optimizations, maintain consistent performance characteristics, and reduce memory usage in the
application layer.

## Layered Authorization

The multi-tenancy system works in conjunction with the external authentication service to provide
layered authorization. While the fulfillment service handles tenant-based visibility through the
`TenancyLogic` interface, the external authentication service can implement additional
authorization rules that determine which specific operations a user is permitted to perform.

For example, the external authentication service might determine that a user can view objects
within certain tenants but cannot modify or delete them. These operation-level permissions are
enforced by the external authentication service before requests reach the fulfillment service,
while tenant-based visibility is enforced by the fulfillment service itself.

This separation of concerns allows for flexible authorization policies that can combine broad
tenant-based access control with fine-grained operation-specific permissions. The external
authentication service can implement complex policies based on user roles, resource types, or
contextual factors, while the fulfillment service focuses on efficient tenant-based data
filtering.

## Implementation Flexibility

The `TenancyLogic` interface is designed to accommodate various multi-tenancy models and
organizational structures. The service includes several implementations that demonstrate different
approaches to tenant management.

The default implementation extracts tenant information from the authenticated user's subject data,
typically mapping users to tenants based on their organizational membership or assigned projects.
This approach works well for scenarios where tenant assignment is relatively static and based on
organizational boundaries.

An empty implementation is also provided, which effectively disables tenant filtering and allows
users to access all objects regardless of tenant assignment. This implementation can be useful
during development, testing, or in single-tenant deployments where multi-tenancy features are not
required.

Custom implementations of the `TenancyLogic` interface can be developed to support more
sophisticated tenant assignment and visibility rules. These might include dynamic tenant
assignment based on request context, hierarchical tenant structures, or integration with external
systems for tenant management.

## Database Schema Considerations

The multi-tenancy implementation requires that all tenant-aware tables include a `tenants` column
of type `text[]` (PostgreSQL text array). This column stores the list of tenant identifiers
associated with each object and serves as the basis for all tenant filtering operations.

The `tenants` column should be indexed to ensure efficient query performance, particularly for
list operations that need to filter large numbers of objects. PostgreSQL's GIN (Generalized
Inverted Index) indexes work well for array columns and can significantly improve the performance
of tenant intersection queries.

The database schema also includes other metadata columns such as `creation_timestamp`,
`deletion_timestamp`, `finalizers`, and `creators`, which work together with the tenants column to
provide comprehensive object lifecycle management within the multi-tenant environment.

## Security Considerations

The multi-tenancy design prioritizes security through several mechanisms. The tenant assignment
and visibility logic operates on authenticated and authorized contexts provided by the external
authentication service, ensuring that tenant determinations are based on verified user identities.

The database-level filtering approach provides defense in depth by ensuring that tenant isolation
is maintained even if application-level logic contains bugs or vulnerabilities. SQL injection
attacks are prevented through the use of parameterized queries, and the tenant filtering logic is
applied consistently across all data access operations.

The system's approach of hiding the existence of inaccessible objects, rather than simply denying
access, helps prevent information leakage that could reveal details about tenant structure or
resource distribution. This is particularly important in multi-tenant SaaS environments where
tenant isolation is critical for compliance and competitive reasons.

## Operational Benefits

From an operational perspective, the multi-tenancy design provides several advantages. The
SQL-based filtering approach ensures predictable performance characteristics that scale with the
overall size of the tenant that a user has access to, rather than the total size of the system.
This means that system performance remains consistent for individual users even as the overall
system grows to accommodate more tenants and data.

The separation between external authentication and internal tenant logic allows for operational
flexibility in managing authentication providers, authorization policies, and tenant structures.
Changes to external authentication configurations do not require modifications to the fulfillment
service, while changes to tenant assignment logic can be implemented through custom `TenancyLogic`
implementations without affecting the external authentication setup.

The architecture also supports various deployment models, from single-tenant installations where
the multi-tenancy features can be disabled, to large-scale multi-tenant SaaS deployments where
sophisticated tenant isolation and performance optimization are critical requirements.

This multi-tenancy design provides a robust foundation for building secure, scalable, and
efficient multi-tenant applications while maintaining the flexibility to adapt to various
organizational structures and operational requirements.
