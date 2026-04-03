import * as admin from "./api-client/admin";
import * as catalog from "./api-client/catalog";
import * as entities from "./api-client/entities";
import * as feed from "./api-client/feed";
import * as feedCore from "./api-client/feed-core";
import * as home from "./api-client/home";
import * as methodology from "./api-client/methodology";
import * as search from "./api-client/search";

export const api = {
  ...catalog,
  ...feedCore,
  ...feed,
  ...search,
  ...home,
  ...entities,
  ...methodology,
  ...admin,
};

export { ApiError } from "./api-client/http";
