/** 向导步骤：选择模板 → 填写参数 → 预览确认。 */
export const wizardSteps = ["select", "params", "preview"] as const;
export type WizardStep = (typeof wizardSteps)[number];

/** 步骤展示元数据。 */
export const wizardStepMeta: Record<
  WizardStep,
  { index: number; label: string }
> = {
  select: { index: 1, label: "选择模板" },
  params: { index: 2, label: "填写参数" },
  preview: { index: 3, label: "预览确认" },
};

export const WIZARD_STEP_COUNT = wizardSteps.length;

/**
 * 解析步骤在流程中的序号（1-based）。
 *
 * @param step - 当前步骤。
 * @returns 序号。
 */
export function wizardStepNumber(step: WizardStep): number {
  return wizardStepMeta[step].index;
}

/**
 * 是否允许进入下一步（纯状态规则，不含异步校验）。
 *
 * @param step - 当前步骤。
 * @param templateId - 已选模板 id。
 * @returns 是否可前进。
 */
export function canGoNext(
  step: WizardStep,
  templateId: string | null,
): boolean {
  if (step === "select") {
    return Boolean(templateId);
  }
  if (step === "params") {
    return Boolean(templateId);
  }
  return false;
}

/**
 * 前进到下一步；非法时返回原步骤。
 *
 * @param step - 当前步骤。
 * @param templateId - 已选模板。
 * @returns 下一步。
 */
export function nextWizardStep(
  step: WizardStep,
  templateId: string | null,
): WizardStep {
  if (!canGoNext(step, templateId)) {
    return step;
  }
  if (step === "select") {
    return "params";
  }
  if (step === "params") {
    return "preview";
  }
  return step;
}

/**
 * 返回上一步；在首步时保持不变。
 *
 * @param step - 当前步骤。
 * @returns 上一步。
 */
export function prevWizardStep(step: WizardStep): WizardStep {
  if (step === "preview") {
    return "params";
  }
  if (step === "params") {
    return "select";
  }
  return "select";
}

/**
 * 向导是否存在未保存工作（用于离开提示）。
 *
 * @param templateId - 已选模板。
 * @param paramsDirty - 参数相对默认值是否变化。
 * @param step - 当前步骤。
 * @returns 是否 dirty。
 */
export function isWizardDirty(
  templateId: string | null,
  paramsDirty: boolean,
  step: WizardStep,
): boolean {
  if (templateId !== null) {
    return true;
  }
  if (paramsDirty) {
    return true;
  }
  return step !== "select";
}
