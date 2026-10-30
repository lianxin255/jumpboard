import React from "react";
export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: "primary" | "secondary" | "quiet" | "danger" | "danger-quiet";
  icon?: string;
}
export declare function Button(props: ButtonProps): React.ReactElement;
