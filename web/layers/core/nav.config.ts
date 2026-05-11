export default {
  moduleId: "core",
  sections: [] as {
    id: string;
    label: string;
    items: {
      id: string;
      label: string;
      icon: string;
      path?: string;
      workspaceId?: string;
      children?: { id: string; label: string; icon: string; path: string; workspaceId?: string }[];
    }[];
  }[]
};
