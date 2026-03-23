const CRUD_NOT_ENABLED_CODE = "CONSULTANT_CRUD_NOT_ENABLED";

function buildDisabledResult(action) {
  return {
    ok: false,
    code: CRUD_NOT_ENABLED_CODE,
    action,
    status: "prepared_only",
    message: "CRUD de consultor ainda nao foi ativado neste MVP."
  };
}

export function createConsultantAdminRepository() {
  return {
    status: "prepared_only",
    enabled: false,
    version: "0.1",
    actions: [
      "listConsultants",
      "createConsultant",
      "updateConsultant",
      "archiveConsultant"
    ],

    async listConsultants() {
      return buildDisabledResult("listConsultants");
    },

    async createConsultant() {
      return buildDisabledResult("createConsultant");
    },

    async updateConsultant() {
      return buildDisabledResult("updateConsultant");
    },

    async archiveConsultant() {
      return buildDisabledResult("archiveConsultant");
    }
  };
}

export { CRUD_NOT_ENABLED_CODE };
