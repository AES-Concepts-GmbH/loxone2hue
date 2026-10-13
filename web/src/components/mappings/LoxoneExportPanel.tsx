import { Download, FileDown } from 'lucide-react';

const linkClass =
  'flex items-center justify-center gap-2 bg-gray-600 hover:bg-gray-500 text-white text-sm font-medium px-4 py-2 rounded-lg transition-colors';

/** Download links for the Loxone Config XML templates (Virtual UDP Input / HTTP Output). */
export function LoxoneExportPanel() {
  return (
    <div className="bg-gray-800 rounded-xl p-6 space-y-4">
      <div className="flex items-center gap-2">
        <Download size={20} className="text-hue-orange" />
        <h3 className="text-lg font-medium text-white">Loxone Config Export</h3>
      </div>
      <p className="text-sm text-gray-400">
        XML-Vorlagen zum Import in Loxone Config. Unter
        <span className="text-gray-300"> Gerätevorlagen &rarr; Vorlage importieren</span> laden.
      </p>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div className="bg-gray-700/50 rounded-lg p-4 space-y-3">
          <div className="flex items-center gap-2">
            <FileDown size={16} className="text-hue-orange" />
            <h4 className="text-sm font-medium text-white">Virtual UDP Input</h4>
          </div>
          <p className="text-xs text-gray-400">
            Empfängt Status-Feedback (On/Off, Helligkeit, Farbtemperatur, Farbe) für gemappte Geräte.
          </p>
          <div className="flex flex-col gap-2">
            <a href="./api/export/inputs" download="VIU_Loxone2HUE.xml" className={linkClass}>
              <Download size={14} />
              Gemappte Geräte
            </a>
            <a href="./api/export/inputs?all=true" download="VIU_Loxone2HUE_all.xml" className={linkClass}>
              <Download size={14} />
              Alle HUE-Geräte
            </a>
          </div>
        </div>

        <div className="bg-gray-700/50 rounded-lg p-4 space-y-3">
          <div className="flex items-center gap-2">
            <FileDown size={16} className="text-hue-orange" />
            <h4 className="text-sm font-medium text-white">Virtual HTTP Output</h4>
          </div>
          <p className="text-xs text-gray-400">
            Steuert HUE-Geräte von Loxone aus (Mood-Befehle, Helligkeit). Basierend auf den Mappings.
          </p>
          <a href="./api/export/outputs" download="VO_Loxone2HUE.xml" className={linkClass}>
            <Download size={14} />
            Ausgänge exportieren
          </a>
        </div>
      </div>
    </div>
  );
}
