import { useAuthStore } from "~/stores/auth";
import { AUTH_TOKEN_COOKIE } from "~/utils/api-client";

export default defineNuxtRouteMiddleware(async (to) => {
  const auth = useAuthStore();
  const isAuthRoute = to.path.startsWith("/auth");
  const accessToken = useCookie(AUTH_TOKEN_COOKIE);
  const hasAccessToken = Boolean(String(accessToken.value || "").trim());

  if (!hasAccessToken) {
    if (isAuthRoute) {
      return;
    }

    return navigateTo(
      {
        path: "/auth/login",
        query: to.fullPath && to.fullPath !== "/" ? { redirect: to.fullPath } : undefined
      },
      { replace: true }
    );
  }

  // Em rotas /auth/* (login/reset/invite), pular o ensureSession evita travar
  // a tela quando o token local esta valido mas o contexto remoto demora.
  // Se o usuario ja esta autenticado, ainda redirecionamos abaixo via
  // auth.isAuthenticated (que reflete o estado local em memoria).
  if (!isAuthRoute) {
    await auth.ensureSession();
  }

  if (isAuthRoute) {
    if (auth.isAuthenticated && to.path === "/auth/login") {
      return navigateTo(auth.mustChangePassword ? "/perfil" : auth.homePath, { replace: true });
    }

    return;
  }

  if (!auth.isAuthenticated) {
    return navigateTo(
      {
        path: "/auth/login",
        query: to.fullPath && to.fullPath !== "/" ? { redirect: to.fullPath } : undefined
      },
      { replace: true }
    );
  }

  if (auth.mustChangePassword && to.path !== "/perfil") {
    return navigateTo("/perfil", { replace: true });
  }

  const workspaceId = String(to.meta.workspaceId || "").trim();
  if (workspaceId && !auth.allowedWorkspaces.includes(workspaceId)) {
    return navigateTo(auth.homePath, { replace: true });
  }
});
