import { describe, expect, it } from "vitest";
import {
  canGoNext,
  isWizardDirty,
  nextWizardStep,
  prevWizardStep,
  wizardStepNumber,
} from "./template-wizard";

describe("template-wizard state machine", () => {
  it("exposes step numbers for progress UI", () => {
    expect(wizardStepNumber("select")).toBe(1);
    expect(wizardStepNumber("params")).toBe(2);
    expect(wizardStepNumber("preview")).toBe(3);
  });

  it("requires template before leaving select", () => {
    expect(canGoNext("select", null)).toBe(false);
    expect(nextWizardStep("select", null)).toBe("select");
    expect(nextWizardStep("select", "postgres-pg-dump")).toBe("params");
  });

  it("moves params → preview and back while preserving flow order", () => {
    expect(nextWizardStep("params", "postgres-pg-dump")).toBe("preview");
    expect(prevWizardStep("preview")).toBe("params");
    expect(prevWizardStep("params")).toBe("select");
    expect(prevWizardStep("select")).toBe("select");
  });

  it("marks dirty once template chosen or params edited", () => {
    expect(isWizardDirty(null, false, "select")).toBe(false);
    expect(isWizardDirty("x", false, "select")).toBe(true);
    expect(isWizardDirty(null, true, "select")).toBe(true);
  });
});
