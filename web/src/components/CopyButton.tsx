import { useEffect, useState, type ReactNode } from 'react';
import { Check, Copy } from 'lucide-react';
import { Tooltip } from './Tooltip';
import { copyToClipboard } from '../utils/clipboard';

interface CopyButtonProps {
  text: string;
  /** Button caption; defaults to "Kopieren". */
  label?: string;
  /** Tooltip and aria-label; defaults to "In Zwischenablage kopieren". */
  tooltip?: string;
  /** Icon shown before the caption; defaults to the copy icon. */
  icon?: ReactNode;
}

/** Copies `text` and shows "Kopiert!" for two seconds. */
export function CopyButton({ text, label = 'Kopieren', tooltip = 'In Zwischenablage kopieren', icon }: CopyButtonProps) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);

  const handleClick = async () => {
    try {
      await copyToClipboard(text);
      setCopied(true);
    } catch (err) {
      console.error('Failed to copy:', err);
    }
  };

  return (
    <Tooltip content={tooltip} position="top">
      <button
        type="button"
        aria-label={tooltip}
        onClick={handleClick}
        className={`px-2 py-1 rounded transition-all flex items-center gap-1 text-xs ${
          copied ? 'bg-green-600 text-white' : 'bg-gray-700 text-gray-400 hover:bg-gray-600 hover:text-white'
        }`}
      >
        {copied ? (
          <>
            <Check size={12} />
            <span>Kopiert!</span>
          </>
        ) : (
          <>
            {icon ?? <Copy size={12} />}
            <span>{label}</span>
          </>
        )}
      </button>
    </Tooltip>
  );
}
