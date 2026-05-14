import { computed } from "vue";
import type { ComputedRef } from "vue";
import {
  AlertTriangle,
  BarChart3,
  Blocks,
  Boxes,
  BrainCircuit,
  Building2,
  CalendarDays,
  ClipboardList,
  Code2,
  Database,
  FileBarChart,
  FileText,
  FormInput,
  Gauge,
  Landmark,
  LayoutPanelLeft,
  Link2,
  ListChecks,
  ListTodo,
  Megaphone,
  MessageCircle,
  MessagesSquare,
  MonitorCog,
  PackageCheck,
  Palette,
  QrCode,
  SearchCheck,
  Settings,
  ShieldCheck,
  Store,
  Users,
  Wrench
} from "lucide-vue-next";
import type { NavItem } from "~/stores/nav";
import { useNavStore } from "~/stores/nav";
import { QUEUE_ONLY_WORKSPACE_IDS } from "~/utils/workspaces";

export const NAV_ICON_MAP: Record<string, unknown> = {
  alert: AlertTriangle,
  audit: ShieldCheck,
  boxes: Boxes,
  brain: BrainCircuit,
  building: Building2,
  calendar: CalendarDays,
  chart: BarChart3,
  database: Database,
  feedback: MessageCircle,
  finance: Landmark,
  forms: FormInput,
  indicators: Gauge,
  integration: Blocks,
  link: Link2,
  manage: LayoutPanelLeft,
  megaphone: Megaphone,
  messages: MessagesSquare,
  monitoring: MonitorCog,
  page: FileText,
  palette: Palette,
  qr: QrCode,
  queue: ListChecks,
  ranking: FileBarChart,
  reports: ClipboardList,
  script: Code2,
  settings: Settings,
  site: PackageCheck,
  stores: Store,
  tasks: ListTodo,
  team: Users,
  tools: Wrench,
  tracking: SearchCheck,
  user: Users,
  users: Users
};

export function useDashboardNav(
  activeWorkspace: ComputedRef<string>,
  allowedWorkspaces: ComputedRef<readonly unknown[]>
) {
  const navStore = useNavStore();
  const route = useRoute();

  const allowedWorkspaceSet = computed(() => new Set(allowedWorkspaces.value || []));
  const currentPath = computed(() => normalizePath(route.path));

  function normalizePath(path: string) {
    return String(path || "").replace(/\/+$/, "") || "/";
  }

  function isItemAllowed(item: NavItem): boolean {
    const workspaceId = String(item.workspaceId || "").trim();
    if (!workspaceId) return true;
    if (!allowedWorkspaceSet.value.has(workspaceId)) return false;
    if (QUEUE_ONLY_WORKSPACE_IDS.has(workspaceId)) return false;
    return true;
  }

  function filterItem(item: NavItem): NavItem | null {
    if (!isItemAllowed(item)) return null;
    if (!Array.isArray(item.children)) return item;
    const children = item.children.filter(isItemAllowed);
    if (!children.length) return null;
    return { ...item, children };
  }

  function isItemActive(item: NavItem): boolean {
    const workspaceId = String(item.workspaceId || "").trim();
    const itemPath = normalizePath(item.path || "");
    if (workspaceId && activeWorkspace.value === workspaceId) return true;
    return Boolean(item.path) && (currentPath.value === itemPath || currentPath.value.startsWith(`${itemPath}/`));
  }

  function isGroupActive(item: NavItem): boolean {
    return Array.isArray(item.children) && item.children.some(isItemActive);
  }

  function resolveIcon(icon: string) {
    return NAV_ICON_MAP[icon] || LayoutPanelLeft;
  }

  const visibleSections = computed(() =>
    navStore.sections
      .map((section) => ({
        ...section,
        items: (section.items || []).map(filterItem).filter((i): i is NavItem => i !== null)
      }))
      .filter((section) => section.items.length > 0)
  );

  const headerItems = computed(() =>
    visibleSections.value.flatMap((section) =>
      section.items.map((item) => ({ ...item, sectionLabel: section.label }))
    )
  );

  return { visibleSections, headerItems, resolveIcon, isItemActive, isGroupActive };
}
