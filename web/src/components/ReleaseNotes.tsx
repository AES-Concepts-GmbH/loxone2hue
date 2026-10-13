import { X, Tag, Wrench, Sparkles, Rocket } from 'lucide-react';
// The add-on changelog is the single source of truth for release notes. It is
// bundled at build time, so the UI can no longer fall behind the CHANGELOG.
import changelog from '../../../loxone2hue/CHANGELOG.md?raw';
import { parseChangelog } from '../utils/changelog';

const releases = parseChangelog(changelog);

function isMinorRelease(version: string): boolean {
  const parts = version.split('.');
  return parts.length === 3 && parts[2] === '0';
}

function getVersionIcon(version: string) {
  if (isMinorRelease(version)) {
    const minor = parseInt(version.split('.')[1], 10);
    if (minor === 0) return <Rocket size={16} className="text-green-400" />;
    return <Sparkles size={16} className="text-hue-orange" />;
  }
  return <Wrench size={14} className="text-gray-400" />;
}

interface ReleaseNotesProps {
  currentVersion: string;
  onClose: () => void;
}

export function ReleaseNotes({ currentVersion, onClose }: ReleaseNotesProps) {
  return (
    <div className="fixed inset-0 bg-black/70 flex items-center justify-center z-50 p-4" onClick={onClose}>
      <div
        className="bg-gray-800 rounded-xl max-w-lg w-full max-h-[80vh] flex flex-col"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-gray-700">
          <div className="flex items-center gap-3">
            <Tag size={20} className="text-hue-orange" />
            <div>
              <h2 className="text-lg font-semibold text-white">Release Notes</h2>
              <p className="text-xs text-gray-400">Aktuelle Version: v{currentVersion}</p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="p-2 text-gray-400 hover:text-white rounded-lg hover:bg-gray-700 transition-colors"
          >
            <X size={18} />
          </button>
        </div>

        {/* Content */}
        <div className="overflow-y-auto px-6 py-4 space-y-4">
          {releases.map((release) => (
            <div key={release.version}>
              <div className="flex items-center gap-2 mb-2">
                {getVersionIcon(release.version)}
                <h3 className={`font-mono font-semibold ${
                  release.version === currentVersion
                    ? 'text-hue-orange'
                    : isMinorRelease(release.version)
                      ? 'text-white'
                      : 'text-gray-300'
                }`}>
                  v{release.version}
                </h3>
                {release.version === currentVersion && (
                  <span className="text-[10px] bg-hue-orange/20 text-hue-orange px-1.5 py-0.5 rounded-full">
                    aktuell
                  </span>
                )}
              </div>
              <ul className="space-y-1 ml-6">
                {release.items.map((item) => (
                  <li key={item} className="text-sm text-gray-400 relative before:content-['•'] before:absolute before:-left-3 before:text-gray-600">
                    {item}
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>

        {/* Footer */}
        <div className="px-6 py-3 border-t border-gray-700">
          <a
            href="https://github.com/AES-Concepts-GmbH/loxone2hue"
            target="_blank"
            rel="noopener noreferrer"
            className="text-xs text-gray-500 hover:text-hue-orange transition-colors"
          >
            github.com/AES-Concepts-GmbH/loxone2hue
          </a>
        </div>
      </div>
    </div>
  );
}
