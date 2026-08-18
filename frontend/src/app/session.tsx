/**
 * Who is "signed in".
 *
 * Cosmetic by design, and worth being blunt about: this app reads one
 * person's local study notes off their own disk. There is no server-side
 * account, no password check, and no data belonging to anyone else to
 * protect. The login screen exists because the interface it is modelled on
 * has one and because opening to a named greeting is nicer than opening to a
 * dashboard — not because anything is being guarded.
 *
 * Kept deliberately obvious rather than dressed up as real auth, so nobody
 * later mistakes it for a security boundary and puts something behind it.
 */

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";

const NAME_KEY = "excelplan:name";

interface SessionValue {
  name: string | null;
  signIn: (name: string) => void;
  signOut: () => void;
}

const SessionContext = createContext<SessionValue | null>(null);

export function SessionProvider({ children }: { children: ReactNode }) {
  const [name, setName] = useState<string | null>(null);

  useEffect(() => {
    try {
      setName(localStorage.getItem(NAME_KEY));
    } catch {
      // Private mode: the app still works, it just asks for a name each time.
    }
  }, []);

  const signIn = useCallback((next: string) => {
    const trimmed = next.trim() || "Alfred";
    setName(trimmed);
    try {
      localStorage.setItem(NAME_KEY, trimmed);
    } catch {
      /* see above */
    }
  }, []);

  const signOut = useCallback(() => {
    setName(null);
    try {
      localStorage.removeItem(NAME_KEY);
    } catch {
      /* see above */
    }
  }, []);

  const value = useMemo<SessionValue>(
    () => ({ name, signIn, signOut }),
    [name, signIn, signOut],
  );

  return <SessionContext.Provider value={value}>{children}</SessionContext.Provider>;
}

export function useSession(): SessionValue {
  const value = useContext(SessionContext);
  if (!value) throw new Error("useSession outside a SessionProvider");
  return value;
}
