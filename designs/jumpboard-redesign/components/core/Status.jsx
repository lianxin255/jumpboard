import React from "react";
export function Status({ children, tone = "neutral" }) {
  return <span className={`jb-status ${tone !== "neutral" ? `jb-status--${tone}` : ""}`}>{children}</span>;
}
