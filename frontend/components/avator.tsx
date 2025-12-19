import { useState } from "react";
import useAuth from "@/lib/auth";
import { useRouter } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";

export default function Avator() {
  const locale = useLocale();
  const t = useTranslations("Avator");
  const [clicked, setClicked] = useState(false);
  const router = useRouter();
  const { isLoading, email, shareCode, loginType, logout } = useAuth();
  if (isLoading) {
    return null;
  }

  const displayName = loginType === "code" ? shareCode : email;
  const displayInitials = displayName.substring(0, 2).toUpperCase();

  return (
    <div className="dropdown dropdown-end">
      <div
        tabIndex={0}
        role="button"
        className="btn btn-ghost btn-circle avatar placeholder"
      >
        <div className="w-8 rounded-full bg-neutral text-neutral-content">
          <span className="text-xs">{displayInitials}</span>
        </div>
      </div>
      <ul
        tabIndex={0}
        className="mt-3 z-[1] p-2 shadow menu menu-sm dropdown-content bg-base-100 rounded-box w-52"
      >
        <li>
          <a className="text-neutral-content btn-disabled">
            <span className="truncate">{displayName}</span>
          </a>
        </li>
        <li>
          <button
            disabled={clicked}
            onClick={async () => {
              setClicked(true);
              await logout();
              router.push(`/${locale}/user/email-code`);
            }}
          >
            {t("logout")}
          </button>
        </li>
      </ul>
    </div>
  );
}
