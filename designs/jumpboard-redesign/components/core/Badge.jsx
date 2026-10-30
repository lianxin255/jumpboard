import React from "react";
export function Badge({ children, protocol, className = "" }) {
  return <span className={`jb-badge ${protocol ? `jb-badge--${protocol.toLowerCase()}` : ""} ${className}`}>{children}</span>;
}
