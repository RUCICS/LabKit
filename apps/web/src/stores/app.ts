import { defineStore } from 'pinia';

export const useAppStore = defineStore('app', {
  state: () => ({
    sidebarOpen: false
  }),
  actions: {
    openSidebar() {
      this.sidebarOpen = true;
    },
    closeSidebar() {
      this.sidebarOpen = false;
    },
    toggleSidebar() {
      this.sidebarOpen = !this.sidebarOpen;
    }
  }
});
