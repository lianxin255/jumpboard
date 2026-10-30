import React from "react";
export interface BadgeProps { children: React.ReactNode; protocol?: "TCP" | "UDP" | "HTTP" | "HTTPS"; className?: string; }
export declare function Badge(props: BadgeProps): React.ReactElement;
