import React from "react";
export function Input({ label, hint, id, className = "", ...props }) {
  return <div className={`jb-field ${className}`}>
    {label && <label htmlFor={id}>{label}</label>}
    <input id={id} className="jb-input" {...props} />
    {hint && <small>{hint}</small>}
  </div>;
}
