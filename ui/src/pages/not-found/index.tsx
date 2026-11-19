// SPDX-License-Identifier: MIT
import { Link } from "react-router-dom";

export default function NotFound() {
  return (
    <div className="page">
      <h1>Page not found</h1>
      <p>
        <Link to="/">Back to My usage</Link>
      </p>
    </div>
  );
}
