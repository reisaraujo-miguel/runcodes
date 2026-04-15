import { createContext, use } from "react";

export const AuthContext = createContext(false);

export const useAuth = () => use(AuthContext);
