import { useState } from 'react';
import { Link2 } from 'lucide-react';
import { CopyButton } from './CopyButton';
import { CredentialUrlPanel } from './CredentialUrlPanel';
import { Tooltip } from './Tooltip';

interface CopyableUrlProps {
  /** Builds the URL for a typed password; an empty one yields the placeholder. */
  buildUrl: (password: string) => string;
  /** Username of the login protection while it is on; then the password is asked for before copying. */
  authUser?: string;
  /** Password shared by all URLs of the page; component state only, never stored. */
  password: string;
  onPasswordChange: (password: string) => void;
  /** Classes for the displayed URL. */
  codeClassName?: string;
}

/**
 * A URL with a copy button. With the login protection on, the button opens
 * the password prompt and the copied URL carries the credentials.
 */
export function CopyableUrl({ buildUrl, authUser, password, onPasswordChange, codeClassName }: CopyableUrlProps) {
  const [open, setOpen] = useState(false);

  return (
    <div>
      <div className="flex flex-wrap items-center gap-2">
        <code className={codeClassName ?? 'bg-gray-900 px-2 py-0.5 rounded font-mono break-all'}>{buildUrl('')}</code>
        {authUser ? (
          <Tooltip content="URL mit Zugangsdaten kopieren (Passwort wird abgefragt, nicht gespeichert)" position="top">
            <button
              type="button"
              aria-expanded={open}
              onClick={() => setOpen((v) => !v)}
              className={`px-2 py-1 rounded transition-all flex items-center gap-1 text-xs ${
                open ? 'bg-gray-600 text-white' : 'bg-gray-700 text-gray-400 hover:bg-gray-600 hover:text-white'
              }`}
            >
              <Link2 size={12} />
              <span>Kopieren</span>
            </button>
          </Tooltip>
        ) : (
          <CopyButton text={buildUrl('')} />
        )}
      </div>
      {authUser && open && (
        <CredentialUrlPanel
          buildUrl={buildUrl}
          authUser={authUser}
          password={password}
          onPasswordChange={onPasswordChange}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  );
}
