import React from "react";

export function Button({ children, variant = "secondary", icon, className = "", ...props }) {
  return <button className={`jb-button jb-button--${variant} ${className}`} {...props}>
    {icon && <img src={icon} alt="" aria-hidden="true" />}
    {children}
  </button>;
}
