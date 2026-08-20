import type { DossierPayload } from "../../services/BackendClient";

export interface DossierExportFile {
  filename: string;
  json: string;
}

export interface DossierExportFeedback {
  severity: "success" | "error";
  message: string;
}

/** One complete, local export: dossier, verification, and reproducibility. */
export function createDossierExport(payload: DossierPayload): DossierExportFile {
  const safeId = payload.dossier_id.replace(/[^a-zA-Z0-9._-]+/g, "-") || "dossier";
  return {
    filename: `decionis-${safeId}.json`,
    json: JSON.stringify(payload, null, 2),
  };
}

export async function copyDossierExport(
  payload: DossierPayload,
  writeText: (json: string) => Promise<void>,
): Promise<DossierExportFeedback> {
  try {
    await writeText(createDossierExport(payload).json);
    return { severity: "success", message: "Dossier JSON copied." };
  } catch {
    return { severity: "error", message: "Dossier JSON could not be copied." };
  }
}

export function downloadDossierExport(
  payload: DossierPayload,
  saveFile: (file: DossierExportFile) => void,
): DossierExportFeedback {
  try {
    saveFile(createDossierExport(payload));
    return { severity: "success", message: "Dossier JSON download started." };
  } catch {
    return { severity: "error", message: "Dossier JSON could not be downloaded." };
  }
}
