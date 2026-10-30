import React from "react";
export function Notice({ children, tone = "warning", action }) {
  return <div className={`jb-notice ${tone === "danger" ? "jb-notice--danger" : ""}`} role={tone === "danger" ? "alert" : "status"}>
    <span className="jb-notice__body">{children}</span>{action}
  </div>;
}
