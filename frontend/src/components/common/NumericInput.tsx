/**
 * Reusable labelled numeric input.
 *
 * Wraps a standard HTML number input with optional label, min/max/step
 * constraints, and configurable width.  Parses the string value on change
 * and only fires the callback when the result is a valid number.
 */
import React from 'react';

interface NumericInputProps {
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  label?: string;
  /** CSS width for the input element (default "80px"). */
  width?: string;
}

const NumericInput: React.FC<NumericInputProps> = ({
  value,
  onChange,
  min,
  max,
  step = 0.01,
  label,
  width = '80px',
}) => {
  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = parseFloat(e.target.value);
    if (!isNaN(val)) {
      onChange(val);
    }
  };

  return (
    <div className="numeric-input">
      {label && <label className="numeric-input-label">{label}</label>}
      <input
        type="number"
        value={value}
        onChange={handleChange}
        min={min}
        max={max}
        step={step}
        style={{ width }}
      />
    </div>
  );
};

export default NumericInput;
