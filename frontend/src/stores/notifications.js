import { defineStore } from 'pinia';
import { ref } from 'vue';

export const useNotificationsStore = defineStore('notifications', () => {
  const items = ref([]);
  let counter = 0;

  function add({ type = 'info', message = '', title = '', duration = 5000, dismissible = true }) {
    const id = ++counter;
    items.value.push({ id, type, message, title, duration, dismissible });
    if (duration > 0) {
      setTimeout(() => remove(id), duration);
    }
    return id;
  }

  function remove(id) {
    const i = items.value.findIndex(n => n.id === id);
    if (i > -1) items.value.splice(i, 1);
  }

  function clear() { items.value = []; }

  return { items, add, remove, clear };
});
