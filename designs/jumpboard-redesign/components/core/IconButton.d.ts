import React from "react";
export interface IconButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  icon: string;
  label: string;
  danger?: boolean;
}
export declare function IconButton(props: IconButtonProps): React.ReactElement;
