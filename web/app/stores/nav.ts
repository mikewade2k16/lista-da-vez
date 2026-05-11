import { defineStore } from "pinia";
import { ref } from "vue";

export interface NavItem {
  id: string;
  label: string;
  icon: string;
  path?: string;
  workspaceId?: string;
  children?: NavItem[];
}

export interface NavSection {
  id: string;
  label: string;
  moduleId: string;
  items: NavItem[];
}

export const useNavStore = defineStore("nav", () => {
  const sections = ref<NavSection[]>([]);

  function register(incoming: NavSection[]) {
    for (const section of incoming) {
      const idx = sections.value.findIndex((s) => s.id === section.id);
      if (idx >= 0) {
        sections.value[idx] = section;
      } else {
        sections.value.push(section);
      }
    }
  }

  function reset() {
    sections.value = [];
  }

  return { sections, register, reset };
});
