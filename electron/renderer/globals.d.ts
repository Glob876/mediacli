interface Window {
  mediacliShell: { version: string };
  mediacliGuard: {
    onQueryActive(callback: () => void): void;
    reportActive(payload: { count: number; lang: string }): void;
  };
  mediacliPreset: { onOpenPresetSettings(callback: () => void): void };
}
