<template>
  <!-- This example requires Tailwind CSS v2.0+ -->
  <div class="my-2 space-y-2 divide-y divide-block-border">
    <div class="flex items-center justify-between">
      <BBTabFilter
        v-if="isCurrentUserDBAOrOwner"
        class="mx-2"
        :tabList="tabItemList.map((item) => item.name)"
        :selectedIndex="state.selectedIndex"
        @select-index="
          (index) => {
            selectTab(index);
          }
        "
      />
      <button
        type="button"
        class="mr-4 btn-normal"
        @click.prevent="markAllAsRead"
      >
        <svg
          class="-ml-1 mr-2 h-5 w-5 text-control-light"
          fill="currentColor"
          viewBox="0 0 20 20"
          xmlns="http://www.w3.org/2000/svg"
        >
          <path
            d="M13.586 3.586a2 2 0 112.828 2.828l-.793.793-2.828-2.828.793-.793zM11.379 5.793L3 14.172V17h2.828l8.38-8.379-2.83-2.828z"
          ></path>
        </svg>
        <span>Mark all as read</span>
      </button>
    </div>
    <div>
      <div class="mt-6 mx-6 space-y-2">
        <div
          class="
            text-lg
            leading-6
            font-medium
            text-main
            pb-4
            border-b border-block-border
          "
        >
          Unread
        </div>
        <InboxList :inboxList="effectiveInboxList(state.unreadList)" />
      </div>
      <div class="mt-6 mx-6 space-y-2">
        <div
          class="
            text-lg
            leading-6
            font-medium
            text-main
            pb-4
            border-b border-block-border
          "
        >
          Read
        </div>
        <InboxList
          class="opacity-70"
          :inboxList="effectiveInboxList(state.readList)"
        />
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { computed, onMounted, reactive, watchEffect } from "vue";
import { useStore } from "vuex";
import InboxList from "../components/InboxList.vue";
import { Inbox, UNKNOWN_ID } from "../types";
import { isDBAOrOwner } from "../utils";
import { useRouter } from "vue-router";

const ISSUE_TAB = 0;
const MEMBER_TAB = 1;

type TabItem = {
  name: string;
  hash: string;
};

const tabItemList: TabItem[] = [
  { name: "Issue", hash: "issue" },
  { name: "Member", hash: "member" },
];

interface LocalState {
  selectedIndex: number;
  readList: Inbox[];
  unreadList: Inbox[];
}

export default {
  name: "Inbox",
  components: { InboxList },
  setup(props, ctx) {
    const store = useStore();
    const router = useRouter();

    const state = reactive<LocalState>({
      selectedIndex: 0,
      readList: [],
      unreadList: [],
    });

    const currentUser = computed(() => store.getters["auth/currentUser"]());

    const selectTabOnHash = () => {
      if (router.currentRoute.value.hash) {
        for (let i = 0; i < tabItemList.length; i++) {
          if (tabItemList[i].hash == router.currentRoute.value.hash.slice(1)) {
            selectTab(i);
            break;
          }
        }
      } else {
        selectTab(ISSUE_TAB);
      }
    };

    const selectTab = (index: number) => {
      state.selectedIndex = index;
      router.replace({
        name: "workspace.inbox",
        hash: "#" + tabItemList[index].hash,
      });
    };

    onMounted(() => {
      selectTabOnHash();
    });

    const prepareInboxList = () => {
      // It will also be called when user logout
      if (currentUser.value.id != UNKNOWN_ID) {
        store
          .dispatch("inbox/fetchInboxListByUser", currentUser.value.id)
          .then((list: Inbox[]) => {
            state.readList = [];
            state.unreadList = [];

            for (const item of list) {
              if (item.status == "READ") {
                state.readList.push(item);
              } else if (item.status == "UNREAD") {
                state.unreadList.push(item);
              }
            }
          });
      }
    };

    watchEffect(prepareInboxList);

    const isCurrentUserDBAOrOwner = computed((): boolean => {
      return isDBAOrOwner(currentUser.value.role);
    });

    const effectiveInboxList = (inboxList: Inbox[]) => {
      return inboxList.filter((inbox: Inbox) => {
        if (
          (state.selectedIndex == ISSUE_TAB &&
            inbox.activity.actionType.startsWith("bb.member.")) ||
          (state.selectedIndex == MEMBER_TAB &&
            !inbox.activity.actionType.startsWith("bb.member."))
        ) {
          return false;
        }
        return true;
      });
    };

    const markAllAsRead = () => {
      var count = state.unreadList.length;
      state.unreadList.forEach((item: Inbox) => {
        store
          .dispatch("inbox/patchInbox", {
            inboxId: item.id,
            inboxPatch: {
              status: "READ",
            },
          })
          .then(() => {
            count--;
            if (count == 0) {
              store.dispatch(
                "inbox/fetchInboxSummaryByUser",
                currentUser.value.id
              );
            }
            state.readList.push(item);
          });
      });
      state.unreadList = [];
    };

    return {
      tabItemList,
      selectTab,
      state,
      isCurrentUserDBAOrOwner,
      effectiveInboxList,
      markAllAsRead,
    };
  },
};
</script>
