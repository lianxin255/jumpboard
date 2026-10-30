import React from "react";
export function NavItem({ icon, label, count, active = false, ...props }) {
  return <button type="button" className="jb-nav-item" aria-current={active ? "page" : undefined} {...props}>
    <img src={icon} alt="" aria-hidden="true" /><span>{label}</span>{count !== undefined && <span className="jb-nav-item__count">{count}</span>}
  </button>;
}
