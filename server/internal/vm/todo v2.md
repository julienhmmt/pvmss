# todo v2

- admin/nodes: we should have tooltips to know what is it for, like the status (activated means it can be used by pvmss users for their VMs)
- admin/storages: same like nodes for tooltips
- admin/isos: same like nodes for tooltips
- admin/isos: we need some filters like sorting, search by name, search by storage, search by node
- admin/isos: i selected an iso but it doesn't show in the list as activated, seems the information is lost on the database nor the backend bug
- admin/bridges: same like isos for the selection
- admin/docs: we should be able to search docs by their name, or slug, or category, its language. also we need some filters for the table
- admin/profiles: we should have some tooltips for the fields, some filters for the table
- admin/tags: we should have some tooltips for the fields, some filters for the table
- admin/pools: we do not need a password field, no password needed to create a pool. a pool creation is just a name and a description. but, the pool creation, needs create a user too. take a look at what we did on the v0.3 app, there is a user creation too and we should have the same workflow
