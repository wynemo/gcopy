import { useSWRConfig } from "swr";
import useSWR from "swr";

interface User {
  email?: string;
  shareCode?: string;
  loggedIn: boolean;
  loginType?: "email" | "code";
}

const defaultUser: User = {
  email: "",
  shareCode: "",
  loggedIn: false,
  loginType: undefined,
};

const userApiRoute = "/api/v1/user";

async function fetcher<JSON>(url: string): Promise<JSON> {
  return fetch(url, {
    headers: {
      accept: "application/json",
      "content-type": "application/json",
    },
  }).then((res) => res.json());
}

export default function useAuth() {
  const { data, isLoading } = useSWR(userApiRoute, fetcher<User>, {
    fallbackData: defaultUser,
  });
  const email = data.email || "";
  const shareCode = data.shareCode || "";
  const loggedIn = data.loggedIn;
  const loginType = data.loginType;
  const { mutate } = useSWRConfig();

  const logout = async () => {
    const response = await fetch("/api/v1/user/logout", {
      headers: {
        accept: "application/json",
        "content-type": "application/json",
      },
    });
    if (response.status == 200) {
      mutate(userApiRoute, defaultUser);
    }
  };

  return { isLoading, email, shareCode, loggedIn, loginType, logout };
}
