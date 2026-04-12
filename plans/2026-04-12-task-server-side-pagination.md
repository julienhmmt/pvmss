# Feature Plan: Server-side Pagination for VM Listings

## Goal
Optimize the performance of the VM list by fetching and rendering only a subset of VMs per page, moving away from fetching all VMs and handling pagination on the client side. This is particularly important for environments with hundreds or thousands of VMs.

## Current State
The application fetches all VMs at once and sends the entire payload to the frontend, which then handles pagination and filtering in the browser.

## Backend Implementation
1. **API Enhancement**: Update the VM list API endpoints (e.g., `GET /api/vms`) to accept query parameters:
   - `page` (default: 1)
   - `limit` or `per_page` (default: 20, max: 100)
   - `search` (optional, for server-side filtering)
   - `sort` and `order` (optional)

2. **Data Slicing Strategy**:
   - Since Proxmox doesn't natively support global cluster-wide pagination in a single API call, we will continue leveraging our backend RAM Cache (`vmCache`).
   - Retrieve all VMs from the cache.
   - Apply any search filters and sorting first.
   - Slice the resulting array in Go: `vms[offset : offset+limit]`.

3. **Response Payload**:
   - Return a standardized pagination wrapper:
     ```json
     {
       "data": [...vms],
       "pagination": {
         "total": 150,
         "page": 1,
         "limit": 20,
         "totalPages": 8
       }
     }
     ```

## Frontend Implementation
1. **Data Fetching**:
   - Update the Svelte/Alpine data fetching logic to append pagination parameters (e.g., `?page=2&limit=20`).
   - Handle the new paginated response structure.

2. **UI Controls**:
   - Add a pagination component at the bottom of the VM table.
   - Include controls for: First Page, Previous, Next, Last Page, and direct page number selection.
   - Add a page size selector (e.g., 10, 20, 50, 100 per page).

3. **State Management**:
   - Manage the current page, page size, and total pages in the client state.
   - Trigger a data refetch when pagination state changes.
   - Update the URL with query parameters (e.g., `?page=2`) so that page state is bookmarkable and survives refreshes.

## Challenges & Considerations
- **Search Integration**: Server-side pagination means search must also move to the server side, otherwise searching only applies to the current page.
- **Cache Staleness**: Ensure the cache is fresh enough that pagination doesn't skip or duplicate items if the cache updates between page requests.
