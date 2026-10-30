import React from "react";
export interface NavItemProps extends React.ButtonHTMLAttributes<HTMLButtonElement> { icon: string; label: string; count?: number; active?: boolean; }
export declare function NavItem(props: NavItemProps): React.ReactElement;
