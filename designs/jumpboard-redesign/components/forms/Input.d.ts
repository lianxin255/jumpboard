import React from "react";
export interface InputProps extends React.InputHTMLAttributes<HTMLInputElement> { label?: string; hint?: string; }
export declare function Input(props: InputProps): React.ReactElement;
