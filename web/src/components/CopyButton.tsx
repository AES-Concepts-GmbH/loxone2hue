import { useEffect, useState } from 'react';
import { Check, Copy } from 'lucide-react';
import { Tooltip } from './Tooltip';
import { copyToClipboard } from '../utils/clipboard';

interface CopyButtonProps {
  text: string;
}

/** Copies `text` and shows "Kopiert!" for two seconds. */
export function CopyButton({ text }: CopyButtonProps) {
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
    <Tooltip content="In Zwischenablage kopieren" position="top">
      <button
        type="button"
        aria-label="In Zwischenablage kopieren"
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
            <Copy size={12} />
            <span>Kopieren</span>
          </>
        )}
      </button>
    </Tooltip>
  );
}
