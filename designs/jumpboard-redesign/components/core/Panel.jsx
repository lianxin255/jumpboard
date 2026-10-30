import React from "react";
export function Panel({ children, className = "", ...props }) {
  return <section className={`jb-panel ${className}`} {...props}>{children}</section>;
}
