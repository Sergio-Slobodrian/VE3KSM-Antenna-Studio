// Copyright 2026 Sergio Slobodrian
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import React, { useEffect, useState } from 'react';
import { getTemplates, generateTemplate } from '@/api/client';
import { useAntennaStore } from '@/store/antennaStore';
import type { Template } from '@/types';

const TemplateSelector: React.FC = () => {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(false);
  const [pending, setPending] = useState<{ name: string; freqMhz: number } | null>(null);
  const { loadTemplate, setFrequency, setError } = useAntennaStore();

  useEffect(() => {
    getTemplates()
      .then(setTemplates)
      .catch(() => {});
  }, []);

  const handleSelect = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const name = e.target.value;
    if (!name) return;
    const tmpl = templates.find((t) => t.name === name);
    if (!tmpl) return;
    const freqParam = tmpl.parameters.find((p) => p.name === 'frequency_mhz');
    setPending({ name, freqMhz: freqParam ? freqParam.default : 146.0 });
    e.target.value = '';
  };

  const handleLoad = async () => {
    if (!pending) return;
    const tmpl = templates.find((t) => t.name === pending.name);
    if (!tmpl) return;

    const params: Record<string, number> = {};
    tmpl.parameters.forEach((p) => { params[p.name] = p.default; });
    params['frequency_mhz'] = pending.freqMhz;

    setLoading(true);
    try {
      const result = await generateTemplate(pending.name, params);
      loadTemplate(result);
      setFrequency({ frequencyMhz: pending.freqMhz });
      setPending(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Template generation failed');
    } finally {
      setLoading(false);
    }
  };

  if (templates.length === 0) return null;

  return (
    <div className="template-selector-wrap">
      <select
        className="template-selector"
        onChange={handleSelect}
        defaultValue=""
        disabled={loading || !!pending}
      >
        <option value="" disabled>
          Load Template
        </option>
        {templates.map((t) => (
          <option key={t.name} value={t.name}>
            {t.name}
          </option>
        ))}
      </select>

      {pending && (
        <div className="template-freq-prompt">
          <span className="template-freq-label">
            {pending.name} — target frequency
          </span>
          <input
            type="number"
            className="template-freq-input"
            value={pending.freqMhz}
            min={0.1}
            step={0.1}
            onChange={(e) =>
              setPending({ ...pending, freqMhz: parseFloat(e.target.value) || pending.freqMhz })
            }
          />
          <span className="template-freq-unit">MHz</span>
          <button
            className="template-freq-btn template-freq-btn--load"
            onClick={handleLoad}
            disabled={loading || !pending.freqMhz || pending.freqMhz <= 0}
          >
            {loading ? 'Loading…' : 'Load'}
          </button>
          <button
            className="template-freq-btn template-freq-btn--cancel"
            onClick={() => setPending(null)}
            disabled={loading}
          >
            Cancel
          </button>
        </div>
      )}
    </div>
  );
};

export default TemplateSelector;
