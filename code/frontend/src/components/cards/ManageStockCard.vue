<template>
  <v-container full-width>
    <v-row>
      <!--Info-->
      <v-col>
        <v-card outlined>
          <v-card-title>{{ item.name }} </v-card-title>
          <v-card-subtitle
            >$ {{ item.price }} <br />
            {{ item.category }}
          </v-card-subtitle>
          <v-card-actions>
            <ManagementDialog
              v-on:form-saved="_updateStockInfo($event)"
              :type="itemType"
              :fields="populateFieldsValues"
              :headline="headline"
            >
              <template v-slot:button="{ on }">
                <v-btn v-show="inStore" v-on="on" color="green" outlined>
                  <v-icon left>mdi-pencil</v-icon>
                  Edit {{ itemType }}
                </v-btn>
                <v-btn
                  color="blue"
                  outlined
                  @click="$emit('toggle-stock', item.itemID)"
                >
                  <v-icon left>mdi-camera-switch</v-icon>
                  {{ stockAction }}</v-btn
                >
              </template>
            </ManagementDialog>
          </v-card-actions>
        </v-card>
      </v-col>
    </v-row>
  </v-container>
</template>

<script>
import ManagementDialog from "../manage/ManagementDialog";

export default {
  name: "ManageItemCard",
  components: {
    ManagementDialog,
  },
  props: {
    item: Object,
    fields: Array,
    stockAction: String,
    inStore: Boolean,
  },
  data() {
    return {
      headline: "Edit Stock",
      itemType: "Stock",
    };
  },
  computed: {
    populateFieldsValues() {
      let filledArray = JSON.parse(JSON.stringify(this.fields));

      for (var field of filledArray) {
        field.value = this.item[`${field.schemaName}`];
      }

      return filledArray;
    },
  },
  methods: {
    mapItemToSchema(fieldArray) {
      let mappedItem = this.item;
      for (var attribute of fieldArray) {
        mappedItem[attribute.schemaName] = attribute.value;
      }

      return mappedItem;
    },
    _updateStockInfo(locationInfo) {
      locationInfo = this.mapItemToSchema(locationInfo);
      locationInfo.itemID = this.item.itemID;
      this.$emit("update", locationInfo);
    },
  },
};
</script>

<style></style>
