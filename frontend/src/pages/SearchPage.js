import { defineComponent, h, ref } from 'vue';
import { useSearchStore } from '../stores/search.js';

function statusClass(s) { return { running: 'is-success', stopped: 'is-danger', paused: 'is-warning' }[s] || 'is-light'; }
function statusIcon(s)  { return { running: 'fa-play', stopped: 'fa-stop', paused: 'fa-pause' }[s] || 'fa-question'; }
function parseTags(str) { return str ? str.split(';').map(t => t.trim()).filter(Boolean) : []; }

export const SearchPage = defineComponent({
  name: 'SearchPage',
  setup() {
    const store = useSearchStore();
    const vmid  = ref('');
    const name  = ref('');
    const tags  = ref('');
    const limit = ref(25);
    let debounceTimer;

    function debounceSearch() {
      clearTimeout(debounceTimer);
      debounceTimer = setTimeout(() => {
        if (vmid.value || name.value || tags.value) {
          store.search({ vmid: vmid.value, name: name.value, tags: tags.value, limit: limit.value });
        } else {
          store.clear();
        }
      }, 400);
    }

    function handleSearch() {
      store.search({ vmid: vmid.value, name: name.value, tags: tags.value, limit: limit.value });
    }

    function handleClear() {
      vmid.value = ''; name.value = ''; tags.value = ''; limit.value = 25;
      store.clear();
    }

    function renderInput(label, model, placeholder, icon) {
      return h('div', { class: 'column' }, h('div', { class: 'field' }, [
        h('label', { class: 'label' }, label),
        h('div', { class: 'control has-icons-left' }, [
          h('input', {
            class: 'input',
            type: 'text',
            value: model.value,
            placeholder,
            onInput: e => { model.value = e.target.value; debounceSearch(); },
            onKeydown: e => { if (e.key === 'Enter') { e.preventDefault(); handleSearch(); } },
          }),
          h('span', { class: 'icon is-small is-left' }, h('i', { class: `fas ${icon}` })),
        ]),
      ]));
    }

    function renderResults() {
      if (!store.hasSearched) return null;
      const rows = store.results.map(vm =>
        h('tr', { key: vm.vmid }, [
          h('td', { class: 'has-text-centered' }, h('span', { class: 'tag is-light is-medium has-text-weight-bold' }, vm.vmid)),
          h('td', h('span', { class: 'has-text-weight-semibold' }, vm.name || '-')),
          h('td', { class: 'has-text-centered' }, h('span', { class: 'tag is-medium' }, vm.node)),
          h('td', { class: 'has-text-centered' }, h('span', { class: `tag is-medium ${statusClass(vm.status)}` }, [
            h('span', { class: 'icon is-small' }, h('i', { class: `fas ${statusIcon(vm.status)}` })),
            ' ', vm.status,
          ])),
          h('td', { class: 'has-text-centered' }, parseTags(vm.tags).map(tag =>
            h('span', { key: tag, class: 'tag is-small is-info is-light mr-1' }, tag)
          )),
          h('td', { class: 'has-text-centered' },
            h('a', { href: `/vm/details/${vm.vmid}`, class: 'button is-primary is-small' }, 'Details')
          ),
        ])
      );

      return h('div', { class: 'card' }, [
        h('header', { class: 'card-header brand-header' },
          h('p', { class: 'card-header-title' }, `Results (${store.results.length})`)
        ),
        h('div', { class: 'card-content' },
          store.results.length === 0 && !store.loading
            ? h('div', { class: 'notification is-warning' }, 'No results found.')
            : h('div', { class: 'table-container' },
                h('table', { class: 'table is-fullwidth is-hoverable' }, [
                  h('thead', h('tr', ['VMID', 'Name', 'Node', 'Status', 'Tags', 'Actions'].map(col =>
                    h('th', { class: 'has-text-centered' }, col)
                  ))),
                  h('tbody', rows),
                ])
              )
        ),
      ]);
    }

    return () => h('section', { class: 'section' },
      h('div', { class: 'container' }, [
        h('div', { class: 'card mb-5' }, [
          h('header', { class: 'card-header brand-header' },
            h('p', { class: 'card-header-title' }, 'Search VMs')
          ),
          h('div', { class: 'card-content' }, [
            h('div', { class: 'columns is-variable is-4' }, [
              renderInput('VM ID', vmid, 'e.g. 100', 'fa-hashtag'),
              renderInput('Name', name, 'e.g. web-server', 'fa-server'),
              renderInput('Tags', tags, 'e.g. pvmss', 'fa-tags'),
            ]),
            h('div', { class: 'buttons' }, [
              h('button', {
                class: ['button', 'is-primary', store.loading && 'is-loading'],
                disabled: store.loading,
                onClick: handleSearch,
              }, store.loading ? 'Searching\u2026' : 'Search'),
              h('button', {
                class: 'button is-light',
                disabled: store.loading,
                onClick: handleClear,
              }, 'Clear'),
            ]),
            store.error && h('div', { class: 'notification is-danger mt-3' }, store.error),
          ]),
        ]),
        renderResults(),
      ])
    );
  },
});
