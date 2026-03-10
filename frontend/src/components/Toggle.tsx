interface ToggleProps {
  enabled: boolean;
  onChange: (enabled: boolean) => void;
  label?: string;
}

export default function Toggle({ enabled, onChange, label }: ToggleProps) {
  return (
    <label class="flex items-center gap-3 cursor-pointer">
      <button
        type="button"
        role="switch"
        aria-checked={enabled}
        onClick={() => onChange(!enabled)}
        class={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
          enabled ? 'bg-amber-600' : 'bg-slate-200'
        }`}
      >
        <span
          class={`inline-block h-4 w-4 rounded-full bg-white transition-transform shadow-sm ${
            enabled ? 'translate-x-6' : 'translate-x-1'
          }`}
        />
      </button>
      {label && <span class="text-sm text-slate-700">{label}</span>}
    </label>
  );
}
