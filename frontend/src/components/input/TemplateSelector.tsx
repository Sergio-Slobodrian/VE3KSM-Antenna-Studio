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

/**
 * Dropdown that loads predefined antenna templates from the backend.
 *
 * On mount, fetches the template list via GET /api/templates.  When the user
 * selects a template, posts its default parameters to GET the generated
 * geometry, then loads it into the store (replacing all wires, source, ground).
 * The select resets to the placeholder after each selection.
 */
import React, { useEffect, useState } from 'react';
import { getTemplates, generateTemplate } from '@/api/client';
import { useAntennaStore } from '@/store/antennaStore';
import type { Template } from '@/types';

const TemplateSelector: React.FC = () => {
  const [templates, setTemplates] = useState<Template[]>([]);
  const [loading, setLoading] = useState(false);
  const { loadTemplate, setError } = useAntennaStore();

  useEffect(() => {
    getTemplates()
      .then(setTemplates)
      .catch(() => {
        // Templates endpoint may not be available
      });
  }, []);

  const handleSelect = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    const name = e.target.value;
    if (!name) return;

    const template = templates.find((t) => t.name === name);
    if (!template) return;

    const params: Record<string, number> = {};
    template.parameters.forEach((p) => {
      params[p.name] = p.default;
    });

    setLoading(true);
    try {
      const result = await generateTemplate(name, params);
      loadTemplate(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Template generation failed');
    } finally {
      setLoading(false);
    }

    e.target.value = '';
  };

  if (templates.length === 0) return null;

  return (
    <select
      className="template-selector"
      onChange={handleSelect}
      defaultValue=""
      disabled={loading}
    >
      <option value="" disabled>
        {loading ? 'Loading...' : 'Load Template'}
      </option>
      {templates.map((t) => (
        <option key={t.name} value={t.name}>
          {t.name}
        </option>
      ))}
    </select>
  );
};

export default TemplateSelector;
