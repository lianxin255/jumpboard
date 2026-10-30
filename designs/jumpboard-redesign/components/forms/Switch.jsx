import React from "react";
export function Switch({ checked, onChange, label, ...props }) {
  return <button type="button" className="jb-switch" role="switch" aria-checked={checked} aria-label={label} onClick={() => onChange(!checked)} {...props}></button>;
}
