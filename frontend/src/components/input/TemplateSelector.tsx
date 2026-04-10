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
