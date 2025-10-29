# Multi-Tenancy Test Plan

## Overview

This test plan provides comprehensive testing coverage for the multi-tenancy feature of the
fulfillment service. The plan ensures that tenant isolation, security, and performance
requirements are thoroughly validated across all system components.

## Scope

This test plan covers **only** the multi-tenancy functionality, including:

- External authentication integration.
- Tenant assignment during object creation.
- Cross-tenant isolation and access control.
- Database-level tenant filtering.
- Object lifecycle with tenant boundaries.

## Test Environment Assumptions

- The fulfillment service is already deployed and configured with Envoy, Authorino and Keycloak.
- The service is configured to use tenant groups from Keycloak.
- Private API access is available for metadata verification.

## Test Cases

### Basic Tenant Isolation

**Cross-Tenant Object Isolation**

This test verifies that users from different tenant groups cannot access each
other's objects.  Ensure that Keycloak contains a user named "alice" that
belongs to a group named "engineering", and also a user "bob" in a "marketing"
group. Login with alice and create a cluster.  Then login with bob and list
clusters. Verify that bob cannot see the cluster created by alice. This
demonstrates that the multi-tenancy system properly isolates objects between
different tenant groups.

**Same-Tenant Object Sharing**

This test confirms that users within the same tenant group can access shared
objects. Ensure that Keycloak contains users "alice" and "charlie" both
belonging to the "engineering" group.  Login with alice and create a cluster
named "shared-cluster". Then login with charlie and list clusters. Verify that
charlie can see and access the cluster created by alice. This confirms that
objects are properly shared within the same tenant group.

**Multi-Tenant User Access**

This test validates that users belonging to multiple tenant groups can access
objects from all their groups. Ensure that Keycloak contains a user "admin" that
belongs to both "engineering" and "marketing" groups, plus users "alice"
(engineering only) and "bob" (marketing only). Have alice create a cluster named
"eng-cluster" and bob create a cluster named "marketing-cluster".  Then login
with admin and list clusters. Verify that admin can see both clusters,
demonstrating proper multi-tenant access.

### Tenant Assignment Verification

**Single-Group Tenant Assignment**

This test verifies that objects created by users are properly assigned to their
tenant groups.  Ensure Keycloak contains a user "alice" in the "engineering"
group. Login with alice and create a cluster named "alice-cluster". Use the
private API to retrieve the cluster object and check the ``metadata.tenants``
field in the response. Verify that "engineering" is listed in the tenants array,
confirming that objects are correctly assigned to the creator's group.

**Multi-Group Tenant Assignment**

This test ensures that objects created by users belonging to multiple groups are
assigned to all their groups. Ensure Keycloak contains a user "admin" in both
"engineering" and "operations" groups. Login with admin and create a host pool
named "admin-pool". Use the private API to retrieve the host pool object and
check the `metadata.tenants` field. Verify that both "engineering" and
"operations" are listed in the tenants array, confirming that multi-group users
have their objects assigned to all their groups.

### Cross-Tenant Operation Restrictions

**Cross-Tenant Update Prevention**

This test verifies that users cannot update objects belonging to other tenant
groups, and that the server responds as if those objects don't exist. Ensure
Keycloak contains users "alice" (engineering group) and "bob" (marketing group).
Have alice create a cluster and note its ID.  Then login with bob and attempt to
update that cluster using its ID directly. Verify that bob receives a "not
found" error rather than a permission denied error, ensuring that the existence
of cross-tenant objects is not revealed.

**Cross-Tenant Deletion Prevention**

This test ensures that users cannot delete objects from other tenant groups,
with the server responding as if the objects don't exist. Using the same user
setup as above, have alice create a cluster, then have bob attempt to delete it
using the cluster ID. Verify that bob receives a "not found" response,
confirming that the system maintains tenant isolation for delete operations and
doesn't reveal the existence of inaccessible objects.

**Cross-Tenant Host Pool Operations**

This test confirms that host pool objects are protected from cross-tenant
modifications. Have alice (engineering group) create a host pool with specific
configuration and note its ID. Then login with bob (marketing group) and attempt
to update that host pool's properties using its ID.  Verify that bob receives a
"not found" response, not a permission denied error, maintaining the illusion
that the object doesn't exist to unauthorized users.

**Cross-Tenant Virtual Machine Access**

This test verifies that virtual machine objects maintain proper tenant
isolation. Have alice create a virtual machine instance and note its ID. Then
have bob attempt to modify the VM's configuration or state using the VM's ID.
Verify that bob receives a "not found" response, ensuring that the existence of
alice's virtual machines is not revealed to users from other tenant groups.

**Cross-Tenant Cluster Operations**

This test ensures that clusters are properly protected from cross-tenant access.
Have alice create a cluster and note its ID. Then login with bob and attempt to
delete that cluster using its ID directly. Verify that bob receives a "not
found" error rather than a permission denied error, confirming that clusters
maintain proper tenant isolation.

### Authentication and Error Handling

**Invalid Authentication Handling**

This test ensures the system properly rejects invalid authentication attempts.
Attempt to access the fulfillment service API using an expired or invalid OAuth
token. Verify that the request is rejected with an appropriate authentication
error and that no service functionality is accessible. The system should not
reveal any information about existing objects or tenant structure in error
responses.

**Unauthenticated Access Restrictions**

This test verifies that unauthenticated requests are properly handled. Attempt
to access various API endpoints without providing any authentication
credentials. Verify that private endpoints return authentication errors while
any public endpoints (if configured) remain accessible. The system should not
leak information about tenant structure or existing objects to unauthenticated
users.

### Complex Scenarios

**Tenant Assignment Persistence**

This test ensures that tenant assignments remain stable during object updates.
Create an object while logged in as a user from a specific tenant group, then
update various properties of that object. Use the private API to verify that the
`metadata.tenants` field remains unchanged after updates. The system should
preserve the original tenant assignments regardless of what fields are modified.

**Large-Scale Tenant Performance**

This test validates system performance when dealing with large numbers of
objects across multiple tenants. Populate the system with hundreds of objects
distributed across multiple tenant groups. Then login as users from different
tenant groups and perform list operations. Measure the response times and verify
that they remain within acceptable limits. The tenant filtering should not
significantly impact performance even with large datasets.

**Concurrent Multi-Tenant Access**

This test ensures the system handles concurrent access from multiple tenant
groups without performance degradation or data leakage. Set up multiple users
from different tenant groups and have them simultaneously perform various
operations (creating, listing, updating objects). Monitor system performance and
verify that each user only sees objects from their authorized tenant groups,
even under concurrent load conditions.

### Security Validation

**Information Leakage Prevention**

This test ensures that no tenant information leaks through error messages or
system responses.  Create objects in various tenant groups, then attempt to
access them using users from different tenant groups. Carefully examine all
error messages, API responses, and system logs to verify that they don't reveal
information about the existence, structure, or content of inaccessible tenant
objects. The system should consistently respond as if inaccessible objects don't
exist.

**Database Query Verification**

This test validates that tenant filtering is enforced at the database level.
Using database administration tools or query logs, examine the SQL queries
generated during various API operations.  Verify that all queries include
appropriate tenant filtering clauses (using PostgreSQL array operations like
`tenants && ...`) and that the database indexes are being used efficiently. This
ensures that tenant isolation is maintained even if application-level logic has
vulnerabilities.

### Edge Cases

**Empty Tenant Group Handling**

This test verifies system behavior when users have no tenant group memberships.
Create a user in Keycloak that doesn't belong to any groups, then attempt to
login and perform various operations.  The system should handle this gracefully,
either by denying access or by applying appropriate default tenant assignments,
depending on the configured tenancy logic.

**End-to-End Workflow Validation**

This test validates complete multi-tenant workflows from authentication through object lifecycle.
Start with users in different tenant groups authenticating through Keycloak, then have them create
clusters using templates, monitor the provisioning process, and eventually clean up resources.
Throughout this workflow, verify that tenant isolation is maintained and that each user only sees
and can interact with objects from their authorized tenant groups.

## Success Criteria

- All cross-tenant isolation tests pass with proper "not found" responses.
- Tenant assignment verification confirms correct `metadata.tenants` values.
- No information leakage detected in any error messages or responses.
- Performance remains acceptable with large multi-tenant datasets.
- Database queries include proper tenant filtering clauses.
- End-to-end workflows maintain tenant boundaries throughout.

## Notes

This test plan focuses exclusively on multi-tenancy functionality and assumes the underlying
service infrastructure is already properly configured and operational. The tests are designed
to be executed against a live system with real Keycloak integration rather than mocked
authentication scenarios.
