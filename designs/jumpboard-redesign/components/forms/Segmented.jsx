import React from "react";
export function Segmented({ options, value, onChange, label }) {
  return <div className="jb-segmented" role="group" aria-label={label}>
    {options.map((option) => <button type="button" className="jb-segment" key={option} aria-pressed={option === value} onClick={() => onChange(option)}>{option}</button>)}
  </div>;
}
