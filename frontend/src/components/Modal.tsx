import type { ComponentChildren } from 'preact';

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ComponentChildren;
}

export default function Modal({ open, onClose, title, children }: ModalProps) {
  if (!open) return null;

  return (
    <div class="fixed inset-0 z-50 flex items-center justify-center">
      <div class="absolute inset-0 bg-black/30 backdrop-blur-sm" onClick={onClose} />
      <div class="relative bg-white rounded-2xl shadow-xl w-full max-w-md mx-4 p-6">
        <div class="flex items-center justify-between mb-4">
          <h2 class="text-lg font-semibold text-slate-800">{title}</h2>
          <button onClick={onClose} class="text-slate-400 hover:text-slate-600 text-xl leading-none">&times;</button>
        </div>
        {children}
      </div>
    </div>
  );
}
