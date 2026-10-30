import React from "react";

export function IconButton({ icon, label, danger = false, className = "", ...props }) {
  return <button type="button" className={`jb-icon-button ${danger ? "jb-icon-button--danger" : ""} ${className}`} aria-label={label} title={label} {...props}>
    <img src={icon} alt="" aria-hidden="true" />
  </button>;
}
